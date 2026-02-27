
```
➜  jed git:(swarm) ✗ mkcert intmon.bn.dev "*.intmon.bn.dev" localhost 127.0.0.1

Created a new certificate valid for the following names 📜
 - "intmon.bn.dev"
 - "*.intmon.bn.dev"
 - "localhost"
 - "127.0.0.1"

Reminder: X.509 wildcards only go one level deep, so this won't match a.b.intmon.bn.dev ℹ️

The certificate is at "./intmon.bn.dev+3.pem" and the key at "./intmon.bn.dev+3-key.pem" ✅

It will expire on 27 May 2028 🗓
```


Setup name resolution and reconfig trfk:
```
➜  jed git:(swarm) ✗ sudo vi /etc/hosts
➜  jed git:(swarm) ✗ ping intmon.bn.dev
PING intmon.bn.dev (127.0.0.1) 56(84) bytes of data.
64 bytes from localhost (127.0.0.1): icmp_seq=1 ttl=64 time=0.029 ms
64 bytes from localhost (127.0.0.1): icmp_seq=2 ttl=64 time=0.027 ms
^C
--- intmon.bn.dev ping statistics ---
2 packets transmitted, 2 received, 0% packet loss, time 1058ms
rtt min/avg/max/mdev = 0.027/0.028/0.029/0.001 ms
➜  jed git:(swarm) ✗ bin/jed_linux-amd64 set-svc scratch/traefik.yaml
set service traefik
➜  jed git:(swarm) ✗ bin/jed_linux-amd64 set-env traefik scratch/traefik.env
set env traefik (9 vars)
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 deploy traefik
deploying traefik (traefik:v3.6.9)
  env: 9 vars
updated traefik
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗ curl https://intmon.bn.dev/whoami
404 page not found
➜  jed git:(swarm) ✗ curl https://intmon.bn.dev/whoami
404 page not found
```
Jiggle whoami cfg:
```
➜  jed git:(swarm) ✗ bin/jed_linux-amd64 set-svc scratch/whoami.yaml
set service whoami
➜  jed git:(swarm) ✗ bin/transship_linux-amd64 deploy whoami
deploying whoami (containous/whoami)
  env: 0 vars
updated whoami
➜  jed git:(swarm) ✗
➜  jed git:(swarm) ✗ curl https://intmon.bn.dev/whoami
404 page not found
➜  jed git:(swarm) ✗ curl https://intmon.bn.dev/whoami
Hostname: f19440190318
IP: 127.0.0.1
IP: ::1
IP: 10.0.1.92
IP: 172.18.0.5
RemoteAddr: 10.0.1.94:33546
GET / HTTP/1.1
Host: intmon.bn.dev
User-Agent: curl/8.18.0
Accept: */*
Accept-Encoding: gzip
X-Forwarded-For: 172.18.0.1
X-Forwarded-Host: intmon.bn.dev
X-Forwarded-Port: 443
X-Forwarded-Prefix: /whoami
X-Forwarded-Proto: https
X-Forwarded-Server: eb1031db275f
X-Real-Ip: 172.18.0.1
```
Boom

