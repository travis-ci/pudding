pudding
======================

[![Build Status](https://travis-ci.org/travis-ci/pudding.svg?branch=master)](https://travis-ci.org/travis-ci/pudding)

[![Deploy](https://www.herokucdn.com/deploy/button.png)](https://heroku.com/deploy)

## Development and such

This repo should be cloned into your `GOPATH` at
`${GOPATH%%:*}/src/github.com/travis-ci/pudding`.
If you don't know what `GOPATH` is or are unsure if the top entry
is in a non-volatile location, you should Ask Someone &trade;

### prerequisites

``` bash
go get github.com/hamfist/deppy
go get github.com/golang/lint/golint
go get code.google.com/p/go.tools/cmd/cover
```

### build/test cycle

Do everything:
``` bash
make
```

Only clean and build, with less output:
```  bash
make clean build GOBUILD_FLAGS=
```

### Running things locally

As with other heroku apps:
``` bash
foreman start
```

The same, but without `rerun` in the mix:
``` bash
DYNO=1 foreman start
```

## Usage

### web

The web API exposes the following resources, with most requiring
authentication via token:

#### `GET /`

Provides a friendly greeting

#### `DELETE /` **requires auth**

Gracefully shut down the server

#### `POST /kaboom` **requires auth**

Simulate a panic.  No body expected.

#### `GET /instances` **requires auth**

Provide a list of instances, optionally filtered with `env`
and `site` query params.

#### `GET /instances/{instance_id}` **requires auth**

Provide a list containing a single instance matching the given
`instance_id`, if it exists.

#### `DELETE /instances/{instance_id}` **requires auth**

Terminate an instance that matches the given `instance_id`, if it
exists.

#### `POST /instance-builds` **requires auth**

Start an instance build, which will result in an EC2 instance being
created.  The expected body is a jsonapi singular collection of
`"instance_build"`, like so:

``` javascript
{
  "instance_builds": {
    "site": "org",
    "env": "staging",
    "instance_type": "c3.2xlarge",
    "count": 4,
    "queue": "docker"
  }
}

```

#### `PATCH /instance-builds/{instance_build_id}` **requires auth**

"Update" an instance build; currently used to send notifications to
Slack upon completion of a build.  Expects
`application/x-www-form-urlencoded` params in the body, a la:

```
state=finished&instance-id=i-abcd1234&slack-channel=general
```

#### `GET /init-scripts/{instance_build_id}` **requires auth**

This route accepts both token auth and "init script auth", which is
basic auth specific to the instance build and is kept in a redis
key with expiry.  This is the route hit by the cloud-init
`#include` set in EC2 instance user data when the instance is
created.  It responds with a content type of `text/x-shellscript;
charset=utf-8`, which is expected (but not enforced) by cloud-init.

#### `GET /images` **requires auth**

Provide a list of images per role, denoting which is active. Example response:

``` javascript
{
  "images": [
    {
      "ami": "ami-00aabbcc",
      "active": true,
      "role": "web"
    },
    {
      "ami": "ami-00aabbcd",
      "active": false,
      "role": "web"
    }
  ]
}
```

### workers

The background job workers are started as a separate process and
communicate with the web server via redis.  The sidekiq-compatible
workers are built using
[`go-workers`](https://github.com/jrallison/go-workers).  There are
also non-evented "mini workers" that run in a simple run-sleep loop
in a separate goroutine.

#### `instance-builds` queue

Jobs handled on the `instance-builds` queue perform the following
actions:

* resolve the `ami` id, using the most recent available if absent
* create a custom security group and authorize inbound port 22
* prepare a cloud-init script and store it in redis
* prepare an `#include` statement with custom URL to be used in the
  instance user-data
* create an instance with the resolved ami id, `#include <url>`
  user-data, custom security group, and specified instance type
* tag the instance with `role`, `Name`, `site`, `env`, and `queue`
* send slack notification that the instance has been created

#### `instance-terminations` queue

Jobs handled on the `instance-terminations` queue perform the
following actions:

* terminate the instance by id, e.g. `i-abcd1234`
* remove the instance from the redis cache
