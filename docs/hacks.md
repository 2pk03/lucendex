### Ask a public server for the current tip index
```bash
NET_TIP=$(curl -s -H "Content-Type: application/json" \
  -d '{"method":"server_info"}' https://s1.ripple.com:51234 \
  | jq -r '.result.info.validated_ledger.seq')
echo "Network tip: $NET_TIP"
```

### For tip and a few behind, fetch hash and request locally
```bash

for L in $(seq "$NET_TIP" -1 $((NET_TIP-9))); do
  H=$(curl -s -H "Content-Type: application/json" \
       -d "{\"method\":\"ledger\",\"params\":[{\"ledger_index\":$L}]}" \
       https://s1.ripple.com:51234 \
     | jq -r '.result.ledger_hash // .result.ledger.hash')
  echo "Requesting $L ($H)"
  docker exec PODNAME /opt/ripple/bin/rippled ledger_request "$H"
done
```
