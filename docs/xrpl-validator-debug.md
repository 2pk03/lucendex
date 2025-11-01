# XRPL Validator Bootstrap & Debug

This guide documents the full debug and bootstrap process for XRPL validator nodes that fail to sync (showing `complete_ledgers: "empty"`).

---

## 1. Initial Cleanup (Nuke and Reconfigure)

Stop the node and clear state:

```bash
docker stop rippled
docker exec rippled rm -rf /var/lib/rippled/db/*
```

Edit `rippled.cfg`:

* Remove `[validator_token]` block.
* Add temporary peers:

  ```ini
  [ips_fixed]
  3.122.10.154 51235
  35.158.172.184 51235
  3.67.69.93    51235
  ```
* Ensure:

  ```ini
  node_size = small
  ledger_history = 256
  online_delete = 512
  ```

Restart:

```bash
docker start rippled
```

---

## 2. Monitor Bootstrap Status

Check sync state:

```bash
docker exec rippled /opt/ripple/bin/rippled server_info \
| jq '.result.info | {server_state, complete_ledgers, peers, io_latency_ms}'
```

Wait until `complete_ledgers` becomes a numeric range (e.g. `99917000-99917300`).

Loop until range acquired:

```bash
while :; do
  r=$(docker exec rippled /opt/ripple/bin/rippled server_info 2>/dev/null \
      | jq -r '.result.info.complete_ledgers // "null"')
  echo "complete_ledgers: $r"
  [[ "$r" =~ ^[0-9]+-[0-9]+$ ]] && echo "range acquired" && break
  sleep 5
done
```

---

## 3. Log-Based Validation

Enable verbose logging for acquisition checks:

```bash
docker exec rippled /opt/ripple/bin/rippled log_level debug
sleep 20
docker exec rippled sh -lc 'tail -n 400 /var/log/rippled/debug.log \
  | egrep -i "accepted ledger|ledger master|Acquiring|Got fetch pack" \
  | tail -n 120'
docker exec rippled /opt/ripple/bin/rippled log_level warning
```

Expected entries:

* `InboundLedger:DBG Trigger acquiring ledger ...`
* `TransactionAcquire:DBG Acquired TX set ...`
* Later: `Accepted ledger ...`

---

## 4. Sync Confirmation

When `complete_ledgers` shows a range:

```bash
docker exec rippled /opt/ripple/bin/rippled server_info \
| jq '.result.info | {server_state, complete_ledgers, peers}'
```

Expect `server_state: "full"`.

---

## 5. Re-Enable Validator Mode

Reinsert your saved `[validator_token]` block at the end of `rippled.cfg`, then restart:

```bash
docker exec rippled /opt/ripple/bin/rippled stop && sleep 2
docker start rippled
```

Verify:

```bash
docker exec rippled /opt/ripple/bin/rippled server_info \
| jq '.result.info | {server_state, complete_ledgers, pubkey_validator, last_close}'
```

Expected:

* `server_state: "proposing"`
* `last_close.proposers > 0`

---

## 6. Final Cleanup

Remove the temporary `[ips_fixed]` block, increase retention:

```ini
[node_db]
online_delete = 2000
```

Restart:

```bash
docker exec rippled /opt/ripple/bin/rippled stop && sleep 2
docker start rippled
```

Validate final state:

```bash
docker exec rippled /opt/ripple/bin/rippled server_info \
| jq '.result.info | {server_state, complete_ledgers, pubkey_validator}'
```

Node should now be proposing with a healthy ledger range.
