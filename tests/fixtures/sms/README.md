# SMS fixtures

The fixture set documents the values exercised by `internal/sms` tests:

- GSM 7-bit full PDU with an SMSC prefix;
- UCS2 Chinese DELIVER TPDU (`你好`);
- 8-bit and 16-bit concatenation metadata;
- out-of-order three-part assembly;
- duplicate-part suppression;
- malformed PDU rejection.

No fixture is taken from a user modem, production database, or SMS inbox.
