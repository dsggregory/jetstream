# Lib Demo
A demo using this library for JetStream access.

## Build for Alpine
This uses docker to build the binary and make it available on the local host filesystem for you to then copy to a k8s pod.

```shell
$ make -f examples/pkg/Makefile docker
```
The docker build creates an alpine binary and copies it to `./jetstream-test` which you can then copy to the nats-box where you run the nats CLI tools.
```shell
$ kubectl cp jetstream-test domains-nats-box-xxx:/root
```

## Run
**Consume**
```shell
./jetstream-test -server nats://domains-nats-0.domains-nats.domains.svc.cl
uster.local:4222 -stream Domains -subject Domains.Domain -f -c 1
```
**Publish**
```shell
./jetstream-test -server nats://domains-nats-0.domains-nats.domains.svc.cl
uster.local:4222 -stream Domains -subject Domains.Domain -p -c 1 'testmsg'
```