# README

## What problem does GoCacheFS resolve?

GoCacheFS can be used to provide a two different filesystem abstractions:

1) A virtual filesystem which _mirrors_ one directory to another. That is, the mountpoint sits infront of two different directories, potentially (and most likely) on two different filesystems. If you write to the mountpoint, the write operation occurs on both backing directories.

2) A virtual filesystem which provides a _caching layer_ on top of a slow filesystem. If you read a file from the mountpoint and it doesn't exist in _dst_, it will be copied from _src_ and stored in _dst_. If you enable writeback support, and write to the mountpoint, the write occurs in a low-latency disk, and then optionally, wrote back to the high-latency filesystem asynchronously. GoCacheFs assumes that writeback operations can fail, and may be configured to retry failed operations. GoCacheFS handles `rm` and `rmdir` by creating hidden empty files in _dst_.

### How to I configure GoCacheFS for mirroring?

`gocachefs -src /dirA -dst /dirB -mountpoint /mnt/mirror -w -concurrent 0`

If you were to then run the following, `foo` would exist in both /dirA and /dirB:

`cp ~/foo /mnt/mirror/`

> NOTE ABOUT INTEGRITY:
>
> foo will first be wrote to _dst_ (dirB) and then _src_ (dirA). GoCacheFS considers the write operation complete when _dst_'s write operation finishes. At this time, _src_ isn't checked for integrity/consistency.
>
> For that reason, it's recommended that you ensure filesystem integrity is maintained by runningly rsync in a nightly cron job or something similiar.

### How to configure GoCacheFS for caching?

Assuming that /src is a remote filesystem with high-latency, and that /dst is a filesystem backed by a SSD:

`gocachefs -src /src -dst /dst -mountpoint /mnt/data`

Let's say you were to do the following:

`cp /mnt/data/foo ~/foo`

If _foo_ did not exist in _dst_, it would be copied from _src_ to _dst_. Any future access to /mnt/data/foo would be done on _dst_, the cached copy of the file.

Then if you were to modify _foo_:

`echo "bar" > /mnt/data/foo`

The file would be modified on /dst/foo. If you enable the writeback operations using _-w_, then the modified file would later by copied back to /src/foo.

## How we use GoCacheFS at Imagely?

For Imagely Sites Hosting, we offload wp-content/uploads to Google Cloud Storage (GCS). We use GoCacheFS to mitigate the high-latency issues of working with GCS.

Example:

`gocachefs -src /mnt/uploads-gcs -dst /mnt/uploads-cache -mountpoint /app/wp-content/uploads -w`

In the example above, /mnt/uploads-gcs is our GCS bucket mounted using rclone (https://github.com/ncw/rclone) and /mnt/uploads-cache is a SSD-backed filesystem.

Additionally, we use the mirror-like capabilities of GoCacheFS for high available persistence storage layer with Kubernetes. If you're interested in learning more, please contact us.

## How to Build

Builds have been tested on both Linux and Mac. Simply run:

`go build -o gocachefs`

## Other notes

### Renaming a directory or file

When a file is renamed, if it doesn't exist on _dst_, it is copied from _src_ to _dst_ first, and then renamed.

## Questions

If you have any questions, contact Michael Weichert:

E-mail: (michael at imagely.com)