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

## Todos
* ~~Make region configurable~~
* Make local port configurable
* Make remote port configurable
* Add flags to skip steps
* Unit testing
* Code cleanup

## Acknowledgements
Most of the connection code is lifted wholesale
from [here](https://github.com/nodefortytwo/amz-ssh).
Thanks for allowing me to be lazy.