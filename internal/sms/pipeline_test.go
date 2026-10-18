package sms

import (
	"context"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"qmi-web/internal/database"
)

func newTestPipeline(t *testing.T) (*Pipeline, *database.Store) {
	t.Helper()
	store, err := database.Open(filepath.Join(t.TempDir(), "qmi-web.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return &Pipeline{Store: store}, store
}

func TestDecodeKnownDeliverPDU(t *testing.T) {
	raw, err := hex.DecodeString("079144872000302320048102020000625061028204401AD9775D0E72D7DBE2B21C949E8360B75A4E7683D16AB71B")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Body == "" || decoded.Sender == "" || decoded.Encoding != "GSM7" {
		t.Fatalf("unexpected decode result: %#v", decoded)
	}
}

func TestDecodeUCS2ChineseAndRejectInvalidPDU(t *testing.T) {
	raw, err := hex.DecodeString("040B912143658709F1000862507241900000044F60597D")
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Body != "你好" || decoded.Encoding != "UCS2" {
		t.Fatalf("unexpected Chinese UCS2 result: %#v", decoded)
	}
	if _, err := Decode([]byte{0x04, 0x01}); err == nil {
		t.Fatal("invalid PDU was accepted")
	}
}

func TestMultipartReassemblyAndDeduplication(t *testing.T) {
	pipeline, store := newTestPipeline(t)
	now := time.Now().UTC()
	part2 := Decoded{Sender: "+15550000001", Body: "world", Timestamp: now, Encoding: "GSM7", Concat: ConcatInfo{IsConcat: true, Ref: 9, RefBits: 8, Total: 2, Seq: 2}}
	part1 := Decoded{Sender: "+15550000001", Body: "hello ", Timestamp: now, Encoding: "GSM7", Concat: ConcatInfo{IsConcat: true, Ref: 9, RefBits: 8, Total: 2, Seq: 1}}
	if _, saved, err := pipeline.IngestDecoded(context.Background(), "mock", 2, []byte("two"), part2); err != nil || !saved {
		t.Fatalf("first part rejected: saved=%v err=%v", saved, err)
	}
	id, saved, err := pipeline.IngestDecoded(context.Background(), "mock", 1, []byte("one"), part1)
	if err != nil || !saved || id < 1 {
		t.Fatalf("reassembly failed: id=%d saved=%v err=%v", id, saved, err)
	}
	message, err := store.SMS(context.Background(), id)
	if err != nil || message.Body != "hello world" || !message.IsMultipart || message.PartsReceived != 2 || message.Source != "test_fixture" {
		t.Fatalf("unexpected message: %#v %v", message, err)
	}
	if _, saved, err := pipeline.IngestDecoded(context.Background(), "mock", 1, []byte("one"), part1); err != nil || saved {
		t.Fatalf("duplicate was persisted: saved=%v err=%v", saved, err)
	}
}

func TestThreePartOutOfOrderReassembly(t *testing.T) {
	pipeline, store := newTestPipeline(t)
	now := time.Now().UTC()
	for _, item := range []struct {
		index uint32
		seq   int
		body  string
	}{
		{3, 3, "three"},
		{1, 1, "one "},
		{2, 2, "two "},
	} {
		decoded := Decoded{
			Sender: "+15550000002", Body: item.body, Timestamp: now, Encoding: "GSM7",
			Concat: ConcatInfo{IsConcat: true, Ref: 513, RefBits: 16, Total: 3, Seq: item.seq},
		}
		_, _, err := pipeline.IngestDecoded(context.Background(), "mock", item.index, []byte{byte(item.seq)}, decoded)
		if err != nil {
			t.Fatal(err)
		}
	}
	items, total, err := store.ListSMS(context.Background(), 1, 10, "", "")
	if err != nil || total != 1 || items[0].Body != "one two three" {
		t.Fatalf("out-of-order three-part message was not assembled: %#v total=%d err=%v", items, total, err)
	}
}

func TestHistoricalMultipartSurvivesRestartAndReferenceReuse(t *testing.T) {
	pipeline, store := newTestPipeline(t)
	ctx := context.Background()
	old := time.Now().UTC().Add(-72 * time.Hour).Truncate(time.Second)
	makePart := func(message, seq int) Decoded {
		body := "same "
		if seq == 2 {
			body = "message"
		}
		return Decoded{Sender: "+15550000003", Body: body, Timestamp: old.Add(time.Duration(message)*time.Hour + time.Duration(seq)*time.Second), Encoding: "GSM7", Concat: ConcatInfo{IsConcat: true, Ref: 7, RefBits: 8, Total: 2, Seq: seq}}
	}
	completed := 0
	pipeline.OnComplete = func(context.Context, database.SMSMessage) { completed++ }
	for message := 0; message < 2; message++ {
		for _, seq := range []int{2, 1} {
			if _, _, err := pipeline.IngestDecoded(ctx, "fixture", uint32(seq), []byte(fmt.Sprintf("message-%d-part-%d", message, seq)), makePart(message, seq)); err != nil {
				t.Fatal(err)
			}
			if message == 0 && seq == 2 {
				// The next process resumes a persisted partial group.
				pipeline = &Pipeline{Store: store, OnComplete: pipeline.OnComplete}
			}
		}
	}
	items, total, err := store.ListSMS(ctx, 1, 10, "", "")
	if err != nil || total != 2 || completed != 2 {
		t.Fatalf("old/reused-reference messages lost: total=%d completed=%d err=%v", total, completed, err)
	}
	for _, item := range items {
		if item.Body != "same message" {
			t.Fatal("assembled the wrong body")
		}
	}
	for message := 0; message < 2; message++ {
		for _, seq := range []int{1, 2} {
			_, saved, err := pipeline.IngestDecoded(ctx, "fixture", uint32(seq), []byte(fmt.Sprintf("message-%d-part-%d", message, seq)), makePart(message, seq))
			if err != nil || saved {
				t.Fatalf("replay duplicated completed message: saved=%v err=%v", saved, err)
			}
		}
	}
	if completed != 2 {
		t.Fatal("replay notified again")
	}
}

func TestAmbiguousMultipartSequencesAreRetainedWithoutMixedInboxRow(t *testing.T) {
	pipeline, store := newTestPipeline(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for i, seq := range []int{1, 1, 2} {
		if _, err := store.AddSMSPart(ctx, "fixture", "sender", 10, 8, 3, seq, now, fmt.Sprintf("body%d", i), fmt.Sprintf("hash%d", i)); err != nil {
			t.Fatal(err)
		}
	}
	last := Decoded{Sender: "sender", Body: "last", Timestamp: now, Encoding: "GSM7", Concat: ConcatInfo{IsConcat: true, Ref: 10, RefBits: 8, Total: 3, Seq: 3}}
	if _, _, err := pipeline.IngestDecoded(ctx, "fixture", 3, []byte("last"), last); err != nil {
		t.Fatal(err)
	}
	_, total, err := store.ListSMS(ctx, 1, 10, "", "")
	if err != nil || total != 0 {
		t.Fatalf("ambiguous group became an inbox message: total=%d err=%v", total, err)
	}
	var pending int
	if err := store.DB().QueryRow("SELECT count(*) FROM sms_parts WHERE assembled=0").Scan(&pending); err != nil || pending != 4 {
		t.Fatalf("ambiguous source parts were lost: count=%d err=%v", pending, err)
	}
}

func TestMultipartRejectsInvalidSequenceWithoutStoringIt(t *testing.T) {
	pipeline, store := newTestPipeline(t)
	decoded := Decoded{Sender: "fixture", Body: "invalid", Timestamp: time.Now(), Concat: ConcatInfo{IsConcat: true, Ref: 3, RefBits: 8, Total: 2, Seq: 3}}
	if _, _, err := pipeline.IngestDecoded(context.Background(), "fixture", 1, []byte("invalid"), decoded); err == nil {
		t.Fatal("accepted invalid multipart sequence")
	}
	var count int
	if err := store.DB().QueryRow("SELECT count(*) FROM sms_parts").Scan(&count); err != nil || count != 0 {
		t.Fatal("invalid part was persisted")
	}
}

func TestReusedReferenceDoesNotMixStaleComplementaryParts(t *testing.T) {
	pipeline, store := newTestPipeline(t)
	ctx := context.Background()
	stamp := time.Now().UTC().Truncate(time.Second)
	ingest := func(raw, body string, seq int, timestamp time.Time) {
		t.Helper()
		decoded := Decoded{Sender: "sender", Body: body, Timestamp: timestamp, Encoding: "GSM7", Concat: ConcatInfo{IsConcat: true, Ref: 11, RefBits: 8, Total: 2, Seq: seq}}
		if _, _, err := pipeline.IngestDecoded(ctx, "fixture", uint32(seq), []byte(raw), decoded); err != nil {
			t.Fatal(err)
		}
	}
	ingest("old-first", "old ", 1, stamp)
	ingest("new-second", "message", 2, stamp.Add(time.Hour))
	_, total, err := store.ListSMS(ctx, 1, 10, "", "")
	if err != nil || total != 0 {
		t.Fatal("joined parts from separate carrier-time windows")
	}
	ingest("new-first", "new ", 1, stamp.Add(time.Hour-time.Second))
	items, total, err := store.ListSMS(ctx, 1, 10, "", "")
	if err != nil || total != 1 || items[0].Body != "new message" {
		t.Fatal("fresh reference reuse could not complete independently")
	}
	var pending int
	if err := store.DB().QueryRow("SELECT count(*) FROM sms_parts WHERE assembled=0").Scan(&pending); err != nil || pending != 1 {
		t.Fatal("discarded the older incomplete message")
	}
}

func TestLegacyMultipartIdentityReplayAndLaterIdenticalBody(t *testing.T) {
	pipeline, store := newTestPipeline(t)
	ctx := context.Background()
	stamp := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Second)
	decoded := Decoded{Sender: "sender", Body: "hello ", Timestamp: stamp, Encoding: "GSM7", Concat: ConcatInfo{IsConcat: true, Ref: 12, RefBits: 8, Total: 2, Seq: 1}}
	ref := 12
	if _, _, err := store.InsertSMS(ctx, database.SMSMessage{DeviceID: "fixture", Sender: "sender", Timestamp: stamp, Body: "hello world", IsMultipart: true, ReferenceNumber: &ref, PartsTotal: 2, RawHash: multipartHash("fixture", decoded, "hello world"), Status: "read"}); err != nil {
		t.Fatal(err)
	}
	completed := 0
	pipeline.OnComplete = func(context.Context, database.SMSMessage) { completed++ }
	for message := 0; message < 2; message++ {
		for seq := 1; seq <= 2; seq++ {
			part := decoded
			part.Concat.Seq = seq
			part.Timestamp = stamp.Add(time.Duration(message) * time.Hour)
			if seq == 2 {
				part.Body = "world"
			}
			if _, _, err := pipeline.IngestDecoded(ctx, "fixture", uint32(seq), []byte(fmt.Sprintf("legacy-replay-%d-%d", message, seq)), part); err != nil {
				t.Fatal(err)
			}
		}
	}
	items, total, err := store.ListSMS(ctx, 1, 10, "", "")
	if err != nil || total != 2 || completed != 1 {
		t.Fatalf("legacy replay/identical later message mismatch: total=%d notifications=%d err=%v", total, completed, err)
	}
	if items[1].Status != "read" {
		t.Fatal("legacy read state changed")
	}
}

func TestOverlappingMultipartScansOnlyPublishOnce(t *testing.T) {
	pipeline, store := newTestPipeline(t)
	ctx := context.Background()
	stamp := time.Now().UTC().Truncate(time.Second)
	var completed atomic.Int32
	pipeline.OnComplete = func(context.Context, database.SMSMessage) { completed.Add(1) }
	var wg sync.WaitGroup
	for scan := 0; scan < 5; scan++ {
		for seq := 1; seq <= 2; seq++ {
			wg.Add(1)
			go func(seq int) {
				defer wg.Done()
				decoded := Decoded{Sender: "sender", Body: fmt.Sprint(seq), Timestamp: stamp, Encoding: "GSM7", Concat: ConcatInfo{IsConcat: true, Ref: 13, RefBits: 8, Total: 2, Seq: seq}}
				if _, _, err := pipeline.IngestDecoded(ctx, "fixture", uint32(seq), []byte(fmt.Sprintf("concurrent%d", seq)), decoded); err != nil {
					t.Error(err)
				}
			}(seq)
		}
	}
	wg.Wait()
	_, total, err := store.ListSMS(ctx, 1, 10, "", "")
	if err != nil || total != 1 || completed.Load() != 1 {
		t.Fatalf("overlapping scans published duplicate SMS: total=%d completed=%d err=%v", total, completed.Load(), err)
	}
}
