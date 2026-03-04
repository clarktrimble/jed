{
  "Name": "whoami",
  "Labels": {
    "traefik.enable": "true",
    "traefik.http.middlewares.whoami-strip.stripprefix.prefixes": "/whoami",
    "traefik.http.routers.whoami.entrypoints": "websecure",
    "traefik.http.routers.whoami.middlewares": "whoami-strip",
    "traefik.http.routers.whoami.rule": "PathPrefix(`/whoami`)",
    "traefik.http.routers.whoami.tls": "true",
    "traefik.http.services.whoami.loadbalancer.server.port": "80"
  },
  "TaskTemplate": {
    "ContainerSpec": {
      "Image": "containous/whoami",
      "User": "1001",
      "ReadOnly": true,
      "Isolation": "default"
    },
    "Resources": {
      "Limits": {
        "NanoCPUs": 500000000,
        "MemoryBytes": 134217728
      },
      "Reservations": {
        "NanoCPUs": 100000000,
        "MemoryBytes": 67108864
      }
    },
    "RestartPolicy": {
      "Condition": "on-failure",
      "Delay": 5000000000,
      "MaxAttempts": 3
    },
    "Networks": [
      {
        "Target": "qn7ll8sv0sikijooyzprx7lpp"
      }
    ],
    "LogDriver": {
      "Name": "json-file",
      "Options": {
        "max-file": "3",
        "max-size": "10m"
      }
    },
    "ForceUpdate": 0,
    "Runtime": "container"
  },
  "Mode": {
    "Replicated": {
      "Replicas": 1
    }
  },
  "UpdateConfig": {
    "Parallelism": 0,
    "FailureAction": "rollback",
    "MaxFailureRatio": 0,
    "Order": "stop-first"
  }
}
