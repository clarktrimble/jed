
```
➜  jed git:(swarm) ✗ bin/jed_linux-amd64 set-svc scratch/traefik.yaml
set service traefik
➜  jed git:(swarm) ✗ bin/jed_linux-amd64 set-env traefik scratch/traefik.env
set env traefik (7 vars)
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 deploy traefik
deploying traefik (traefik:v3.6.9)
  env: 7 vars
created traefik (wp5ojx06w6ek)
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 tasks traefik
2026-02-27T21:57:27     iwu63doym56q    running traefik:v3.6.9
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 logs iwu63doym56q
2026-02-27T21:57:27Z ERR Failed to retrieve information of the docker client and server host error="permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock: Get \"http://%2Fvar%2Frun%2Fdocker.sock/v1.51/version\": dial unix /var/run/docker.sock: connect: permission denied" providerName=docker
```
Add ability to set uid/gid and:
```
➜  jed git:(swarm) ✗ bin/jed_linux-amd64 set-svc scratch/traefik.yaml
set service traefik
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 deploy traefik
deploying traefik (traefik:v3.6.9)
  env: 7 vars
updated traefik
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 tasks traefik
2026-02-27T22:07:32     zejz2u4m7wbt    running traefik:v3.6.9
2026-02-27T22:07:31     iwu63doym56q    shutdown        traefik:v3.6.9
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 logs zejz2u4m7wbt
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 logs zejz2u4m7wbt
```
Webui is up on http://localhost:8080 !

Try routing some traffic:
```
➜  jed git:(swarm) ✗ bin/jed_linux-amd64 set-svc scratch/whoami.yaml
set service whoami
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 deploy whoami
deploying whoami (containous/whoami)
  env: 0 vars
created whoami (dhgtqt27zv3z)
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 tasks whoami
2026-02-27T22:24:48     mkucrdi97cuc    running containous/whoami
➜  jed git:(swarm) ✗ curl -H "Host: whoami.localhost" http://localhost/
404 page not found
```
Switch from DOCKER to SWARM provider in traefik:
```
➜  jed git:(swarm) ✗ bin/jed_linux-amd64 set-env traefik scratch/traefik.env
set env traefik (7 vars)
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 deploy traefik
deploying traefik (traefik:v3.6.9)
  env: 7 vars
updated traefik
➜  jed git:(swarm) ✗ curl -H "Host: whoami.localhost" http://localhost/
Hostname: f19440190318
IP: 127.0.0.1
IP: ::1
IP: 10.0.1.92
IP: 172.18.0.5
RemoteAddr: 10.0.1.93:37940
GET / HTTP/1.1
Host: whoami.localhost
User-Agent: curl/8.18.0
Accept: */*
Accept-Encoding: gzip
X-Forwarded-For: 172.18.0.1
X-Forwarded-Host: whoami.localhost
X-Forwarded-Port: 80
X-Forwarded-Proto: http
X-Forwarded-Server: ba0bdc853acc
X-Real-Ip: 172.18.0.1
```
Woot

Now with path prefix routing:
```
➜  jed git:(swarm) ✗ bin/jed_linux-amd64 set-svc scratch/whoami.yaml
set service whoami
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 deploy whoami
deploying whoami (containous/whoami)
  env: 0 vars
updated whoami
➜  jed git:(swarm) ✗ curl http://localhost/whoami
404 page not found
➜  jed git:(swarm) ✗ curl http://localhost/whoami
Hostname: f19440190318
IP: 127.0.0.1
IP: ::1
IP: 10.0.1.92
IP: 172.18.0.5
RemoteAddr: 10.0.1.93:49212
GET / HTTP/1.1
Host: localhost
User-Agent: curl/8.18.0
Accept: */*
Accept-Encoding: gzip
X-Forwarded-For: 172.18.0.1
X-Forwarded-Host: localhost
X-Forwarded-Port: 80
X-Forwarded-Prefix: /whoami
X-Forwarded-Proto: http
X-Forwarded-Server: ba0bdc853acc
X-Real-Ip: 172.18.0.1
```
