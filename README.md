# tunneller

A small application to make it easier to get to
RDS servers in private subnets via EC2 bastion
servers

## Requirements
* make
* golang

## Building
Run `make build`, a binary will be produced
in the `build` folder

## Running
First, configure your AWS `~/.aws/credentials` file. Tunneller
will read this during execution. At this point, it doesn't
support custom credential file locations, so ensure it's
in the correct place.

Tunneller has its own terminal based UI, so it can be run
by simply invoking the binary. There are, however a few flags
that can be used to skip a few steps:
* `-profile` - The profile name to use
* `-local-port` - Which local port to bind to, default is 8888
* `-region` - Which AWS region to use

## How it works
Tunneller uses the `ec2-instance-connect` part of the AWS SDK
to upload a public key into the selected EC2 instance and then
does a vanilla SSH tunnel through. The equivalent CLI commands
would be
```
ssh-keygen -f bastionkey -N ""
aws ec2-instance-connect send-ssh-public-key \
	  --instance-id $(BASTION_ID) \
	  --instance-os-user $(BASTION_USER) \
	  --ssh-public-key file://./bastionkey.pub \
	  --availability-zone $(BASTION_AZ)
ssh -L -L 8888:$(DB_HOST):$(DB_PORT) ec2-user@$(BASTION_USER)
```

## Todos
* ~~Make region configurable~~
* ~~Make local port configurable~~
* ~~Make remote port configurable~~
* ~~Add flags to skip steps~~
* Unit testing
* Code cleanup
* Make bastion username configurable
* CI pipeline to compile and package binary

## Acknowledgements
Most of the connection code is lifted wholesale
from [here](https://github.com/nodefortytwo/amz-ssh).
Thanks for allowing me to be lazy.