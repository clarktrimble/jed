{
  "Name": "traefik",
  "Labels": {},
  "TaskTemplate": {
    "ContainerSpec": {
      "Image": "traefik:v3.6.9",
      "Env": [
        "TRAEFIK_ACCESSLOG=true",
        "TRAEFIK_API_DASHBOARD=true",
        "TRAEFIK_API_INSECURE=true",
        "TRAEFIK_ENTRYPOINTS_WEB_ADDRESS=:80",
        "TRAEFIK_PROVIDERS_FILE_FILENAME=/certs/tls.yml",
        "TRAEFIK_PROVIDERS_SWARM=true",
        "TRAEFIK_PROVIDERS_SWARM_EXPOSEDBYDEFAULT=false",
        "TRAEFIK_PROVIDERS_SWARM_NETWORK=svc-net",
        "TRAEFIK_ENTRYPOINTS_WEBSECURE_ADDRESS=:443"
      ],
      "User": "1000:967",
      "ReadOnly": true,
      "Mounts": [
        {
          "Type": "bind",
          "Source": "/home/trimble/ctlib/jed/scratch/certs",
          "Target": "/certs"
        },
        {
          "Type": "bind",
          "Source": "/var/run/docker.sock",
          "Target": "/var/run/docker.sock"
        }
      ],
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
  },
  "EndpointSpec": {
    "Mode": "vip",
    "Ports": [
      {
        "Protocol": "tcp",
        "TargetPort": 443,
        "PublishedPort": 443,
        "PublishMode": "host"
      },
      {
        "Protocol": "tcp",
        "TargetPort": 80,
        "PublishedPort": 80,
        "PublishMode": "host"
      },
      {
        "Protocol": "tcp",
        "TargetPort": 8080,
        "PublishedPort": 8080,
        "PublishMode": "host"
      }
    ]
  }
}
