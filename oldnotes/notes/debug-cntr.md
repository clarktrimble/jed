
```
➜  jed git:(swarm) ✗ go run cmd/transship/main.go create-network svc-net
error: failed to create network "svc-net": http POST request to http://localhost /v1.52/networks/create failed: Post "http://localhost/v1.52/networks/create": unexpected status code 409 with body: {"message":"network with name svc-net already exists"}

exit status 1
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗ go run cmd/jed/main.go -h
Usage: main [--db DB] <command> [<args>]

Options:
  --db DB, -d DB         path to bbolt database [default: jed.db]
  --help, -h             display this help and exit

Commands:
  ls-svc                 list services
  get-svc                get a service
  set-svc                set a service from YAML
  ls-env                 list envs
  get-env                get env for a service
  set-env                set env from .env file
➜  jed git:(swarm) ✗ go run cmd/jed/main.go ls-svc
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗ go run cmd/jed/main.go set-svc test/data/svcenv/debug.yaml
set service debug
➜  jed git:(swarm) ✗ go run cmd/jed/main.go set-env debug test/data/svcenv/debug.env
set env debug (0 vars)
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗ go run cmd/transship/main.go deploy debug
deploying debug (alpine:latest)
  env: 0 vars
created debug (oyjrmow65scj)
```
