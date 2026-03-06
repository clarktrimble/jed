```
➜  ~ ssh bnoperator@172.21.106.20
bnoperator@172.21.106.20's password:

BastilleNetworks, Inc

Last login: Thu Mar  5 13:31:14 2026 from 172.16.2.31
Bastille Networks
[Bastille / fusioncenter] >> file edit network_services.yml
[Bastille / fusioncenter] >>
Connection to 172.21.106.20 closed.
➜  ~ ssh bnoperator@172.21.106.10
The authenticity of host '172.21.106.10 (172.21.106.10)' can't be established.
ECDSA key fingerprint is: SHA256:3n8TjyAQtBg9UUla5lO+P04N6aJutDeRIaEmGRW4Ssc
This key is not known by any other names.
Are you sure you want to continue connecting (yes/no/[fingerprint])? yes
Warning: Permanently added '172.21.106.10' (ECDSA) to the list of known hosts.
bnoperator@172.21.106.10's password:

BastilleNetworks, Inc

Last login: Tue Mar  3 18:49:22 2026 from 192.168.250.2
Bastille Networks
[Bastille / networkservices] >> file edit network_services.yml
[Bastille / networkservices] >> ls
file list
Usage: file list [-h] [-a] [-l] [-r] [-s {size, time}] {code, config, home, system_logs, certs, netplan} [target]
Error: the following arguments are required: namespace

[Bastille / networkservices] >> ls config
file list config
etc  packages
[Bastille / networkservices] >> ls home
file list home
network-services.yml  sample.yml  01-netcfg.yaml.bak
[Bastille / networkservices] >> file edit network-services.yml
[Bastille / networkservices] >> install network_services -c network_services.yml
EXCEPTION of type 'ValueError' occurred with message: path network_services.yml is outside namespace home or does not exist
[Bastille / networkservices] >> install network_services -c network-services.yml
[+] Reading config
[+] Writing DNS addresses
Synchronizing state of dnsmasq.service with SysV service script with /lib/systemd/systemd-sysv-install.
Executing: /lib/systemd/systemd-sysv-install enable dnsmasq
[+] Writing DHCP configuration
[+] Enabling NTP
Synchronizing state of ntp.service with SysV service script with /lib/systemd/systemd-sysv-install.
Executing: /lib/systemd/systemd-sysv-install enable ntp

[+] Setup complete. Please reboot to start all services
[Bastille / networkservices] >> system reboot now
Connection to 172.21.106.10 closed by remote host.
Connection to 172.21.106.10 closed.
➜  ~ ssh bnoperator@172.21.106.10
ssh: connect to host 172.21.106.10 port 22: Connection refused
➜  ~ ssh bnoperator@172.21.106.10
bnoperator@172.21.106.10's password:

BastilleNetworks, Inc

Last login: Thu Mar  5 15:47:52 2026 from 172.16.2.32
Bastille Networks
[Bastille / networkservices] >> help

Documented commands (use 'help -v' for verbose/'help <topic>' for details):

Connectivity
============
http  network

Operations
==========
install  logs  process  service  system  update

Session
=======
help  quit  terminal

Settings
========
config  file  firewall  time

Undocumented commands:
======================
exit

[Bastille / networkservices] >> network ping
ping     ping6
[Bastille / networkservices] >> network ping intmon
ping: intmon: Name or service not known
error pinging remote system
[Bastille / networkservices] >> network ping intmon.vadc-test.bastille.cloud
PING intmon.vadc-test.bastille.cloud (10.35.44.122) 56(84) bytes of data.
64 bytes from 10.35.44.122 (10.35.44.122): icmp_seq=1 ttl=63 time=0.573 ms
64 bytes from 10.35.44.122 (10.35.44.122): icmp_seq=2 ttl=63 time=0.527 ms
64 bytes from 10.35.44.122 (10.35.44.122): icmp_seq=3 ttl=63 time=0.400 ms
^C
--- intmon.vadc-test.bastille.cloud ping statistics ---
3 packets transmitted, 3 received, 0% packet loss, time 2019ms
rtt min/avg/max/mdev = 0.400/0.500/0.573/0.073 ms
```
