# Managing autoscaling bits

A new autoscaling group may be defined by specifying only the name
of the autoscaling group and an existing instance id.  Without
specifying the tags, only the autoscaling group name tag will be
applied, though, so instead tags should be passed at autoscaling
group creation time, e.g.:

``` bash
aws autoscaling create-auto-scaling-group \
  --instance-id i-2a32e3cb \
  --tags \
    Key=role,Value=worker \
    Key=queue,Value=testing \
    Key=site,Value=org \
    Key=env,Value=staging \
    Key=Name,Value=travis-org-staging-testing-2a32e3cb-asg \
  --min-size 1 \
  --max-size 3 \
  --desired-capacity 1 \
  --auto-scaling-group-name 2a32e3cb-asg
```

## Problems:

### user-data init script lifespan

The `user-data` for the instances created by the worker manager
service is currently doing an `#include` of an init script URL
that's intented to be short-lived.  If this user-data is going to
be used within an auto-scaling context, then we'll either have to
know up front and give it a considerably longer expiry (or no
expiry at all), or perhaps remove expiry from init-scripts and
their auths altogether.

### Name tags no longer unique

The `Name` tag for instances within an autoscaling group cannot (?)
be based on the instance id, e.g.
`travis-org-staging-docker-abcd1234`.  One option is to do like the
above `aws autoscaling create-auto-scaling-group` invocation and
assign a name that ends with the root instance id and `-asg`, but
then individual instances within the autoscaling group will not be
unique.  This may require setting the system hostname dynamically
during cloud init so that it includes the instance id fetched from
the metadata API.

### Managing autoscaling policies

Much of the literature is geared toward web applications that live
behind ELB.  We may have to manually manage instances within
autoscaling groups, or rely on coarse-grained health checks like
cpu utilization, e.g.:

``` bash
# Create a scale-out Policy
aws autoscaling put-scaling-policy \
  --policy-name 2a32e3cb-sop \
  --auto-scaling-group-name 2a32e3cb-asg \
  --adjustment-type ChangeInCapacity \
  --scaling-adjustment 1 > out.json

POLICY_ARN=$(jq .PolicyARN < out.json)

# Assign the scale-out policy to a cloud watch alarm for
# CPUUtilization by arn
aws cloudwatch put-metric-alarm \
  --alarm-name 2a32e3cb-add-capacity \
  --metric-name CPUUtilization \
  --namespace AWS/EC2 \
  --statistic Average \
  --period 120 \
  --threshold 95 \
  --comparison-operator GreaterThanOrEqualToThreshold \
  --dimensions Name=AutoScalingGroupName,Value=2a32e3cb-asg \
  --evaluation-periods 2 \
  --alarm-actions "$POLICY_ARN"
```

Alternatively, we could expose a small web API on `travis-worker`
to take advantage of ELB-based health checks.

### Graceful shutdown is hard

By default, scale-in policies are very coarse grained and will
result in a `shutdown`/`halt`/`poweroff`, meaning there will not be
an opportunity to gracefully wait for long-running jobs to finish.
It is not possible to accurately anticipate which instance in an
autoscaling group will be targeted for termination during a
scale-in, so there's not much we can do to make this any more
graceful.  Again, it seems like the system is geared toward web
servers with short-lived connections.

