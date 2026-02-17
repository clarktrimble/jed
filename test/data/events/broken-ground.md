
# after deploy

Hard to see what's happening via events:

1. Create: service/create + container/start, no updatestate
2. Noop: single service/update, no updatestate, nothing else
3. Label-only: service/update, no container recreation, no updatestate
4. Real change: updatestate.new: "updating" → "completed"

## nice to watch

With
```
curl -s --unix-socket /var/run/docker.sock http://localhost/v1.52/events
```

