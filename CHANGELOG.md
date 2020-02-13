## Changelog

### v1.2 - Jan 4, 2019
- Fixed problem with worker utilizing 100% CPU

### v1.1 - Nov 30, 2018
- Added retry-delay option. Wait the number of specified seconds before retrying a failed writeback operation
- Added start-delay option. Wait the number of specified seconds before starting a writeback operation
- Added the display of job status information if the process receives a SIGUSR1 signal

### v1.0 - Nov 30 2018
- Fixed concurrency model. Previously write back operations were being queued until the number of jobs was equal to the value of the _concurrent_ option specified. The desired effect is to run jobs as soon as they become unavailable and only start queuing jobs once the number of active jobs is equal to the value of the _concurrent_ option.

### v0.1 - Nov 15 2018
- Initial release