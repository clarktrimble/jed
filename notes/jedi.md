
# jedi
A docker mgmt web UI.
Todo: more about purpose/rationale/reason-de-tor

## from other doc
o service creation: start from image and mostly traefik labels (in jedi)
o env creation: start with help from service if available (in jedi)

## contstraints
- single node
- rely on local docker images
- interact with docker via jed
- terminate external connections via traefik (not constraint, Todo: traefik section)

## service

Todo: what is service

### create
- choose image
- specify unique name suffix
- choose restart policy
- specify path for traefik routing (opt)
- create env (opt)

### deploy / undeploy
- stop/start button

### logs
- page showing recent logs (refresh)

### delete
- button (with "are you sure")

## env

Todo: what is env

### creation
- choose service
- specify key/val pairs
- optionally populate via api help (if available)
- optionally validate via api check (if available)
- allow for upload/text edit???

### update
- edit key/val pairs
- optionally validate via api check (if available)
- save via redeploy button


