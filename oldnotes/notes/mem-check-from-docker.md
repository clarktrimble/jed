
c5a➜  deploy docker stats --no-stream --format "{{.MemUsage}}" $(docker ps -qf name=reauth-acp)
3.562MiB / 256MiB
