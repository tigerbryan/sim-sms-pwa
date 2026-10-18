-- Reference numbers are reused by senders. Preserve each PDU's identity and
-- remember completed parts so they cannot be mixed into the next message.
ALTER TABLE sms_parts RENAME TO sms_parts_legacy;
CREATE TABLE sms_parts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  device_id TEXT NOT NULL,
  sender TEXT NOT NULL,
  concat_reference INTEGER NOT NULL,
  concat_reference_bits INTEGER NOT NULL,
  total_parts INTEGER NOT NULL,
  sequence_number INTEGER NOT NULL,
  received_at TEXT NOT NULL,
  body TEXT NOT NULL,
  raw_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  assembled INTEGER NOT NULL DEFAULT 0,
  UNIQUE(device_id, raw_hash)
);
INSERT INTO sms_parts(id,device_id,sender,concat_reference,concat_reference_bits,total_parts,sequence_number,received_at,body,raw_hash,created_at)
SELECT id,device_id,sender,concat_reference,concat_reference_bits,total_parts,sequence_number,received_at,body,raw_hash,created_at
FROM sms_parts_legacy;
DROP TABLE sms_parts_legacy;
CREATE INDEX idx_sms_parts_pending ON sms_parts(device_id,sender,concat_reference,concat_reference_bits,total_parts,assembled,received_at);

-- Preserve ownership of old completed groups during the upgrade. The former
-- schema allowed at most one part per sequence for each reference group.
UPDATE sms_parts AS part SET assembled=1 WHERE EXISTS (
  SELECT 1 FROM sms_messages AS message
  WHERE message.device_id=part.device_id AND message.sender=part.sender
    AND message.is_multipart=1 AND message.reference_number=part.concat_reference
    AND message.parts_total=part.total_parts
    -- Old inbox rows have no reference-bit-width column. If both widths are
    -- present, leave ownership to the runtime's exact legacy-hash comparison.
    AND NOT EXISTS (SELECT 1 FROM sms_parts AS other WHERE other.device_id=part.device_id AND other.sender=part.sender AND other.concat_reference=part.concat_reference AND other.total_parts=part.total_parts AND other.concat_reference_bits<>part.concat_reference_bits)
    AND message.timestamp BETWEEN
      (SELECT min(received_at) FROM sms_parts AS p WHERE p.device_id=part.device_id AND p.sender=part.sender AND p.concat_reference=part.concat_reference AND p.concat_reference_bits=part.concat_reference_bits AND p.total_parts=part.total_parts)
      AND
      (SELECT max(received_at) FROM sms_parts AS p WHERE p.device_id=part.device_id AND p.sender=part.sender AND p.concat_reference=part.concat_reference AND p.concat_reference_bits=part.concat_reference_bits AND p.total_parts=part.total_parts)
    AND part.total_parts=(SELECT count(*) FROM sms_parts AS p WHERE p.device_id=part.device_id AND p.sender=part.sender AND p.concat_reference=part.concat_reference AND p.concat_reference_bits=part.concat_reference_bits AND p.total_parts=part.total_parts)
    AND message.body=(SELECT group_concat(body,'') FROM (
      SELECT body FROM sms_parts AS p WHERE p.device_id=part.device_id AND p.sender=part.sender AND p.concat_reference=part.concat_reference AND p.concat_reference_bits=part.concat_reference_bits AND p.total_parts=part.total_parts ORDER BY sequence_number
    ))
);
