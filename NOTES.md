# Managing autoscaling bits

A new autoscaling group may be defined by specifying only the name of the autoscaling group and an existing instance id.
Without specifying the tags, only the autoscaling group name tag will be applied, though, so instead tags should be
passed at autoscaling group creation time, e.g.:

``` bash
aws autoscaling create-auto-scaling-group \
  --instance-id i-$INSTANCE_ID \
  --tags \
    Key=role,Value=worker \
    Key=queue,Value=docker \
    Key=site,Value=org \
    Key=env,Value=staging \
    Key=Name,Value=org-staging-docker-asg-$INSTANCE_ID \
  --min-size 1 \
  --max-size 3 \
  --desired-capacity 1 \
  --auto-scaling-group-name org-staging-docker-asg-$INSTANCE_ID
```

Each autoscaling group will need both scale-in and scale-out policies, e.g. scale out:

``` bash
aws autoscaling put-scaling-policy \
  --policy-name org-staging-docker-sop-$INSTANCE_ID \
  --auto-scaling-group-name org-staging-docker-asg-$INSTANCE_ID \
  --adjustment-type ChangeInCapacity \
  --scaling-adjustment 1
```

and scale in:

``` bash
aws autoscaling put-scaling-policy \
  --policy-name org-staging-docker-sip-$INSTANCE_ID \
  --auto-scaling-group-name org-staging-docker-asg-$INSTANCE_ID \
  --adjustment-type ChangeInCapacity \
  --scaling-adjustment -1
```

The above call responds with a policy ARN which must be used when assigning the metric alarm, e.g. scale out:

``` bash
aws cloudwatch put-metric-alarm \
  --alarm-name org-staging-docker-$INSTANCE_ID-add-capacity \
  --metric-name 'v1.travis.rabbitmq.queues.builds.docker.messages_ready' \
  --namespace Travis/org \
  --statistic Maximum \
  --period 120 \
  --threshold 1 \
  --comparison-operator GreaterThanOrEqualToThreshold \
  --dimensions Name=AutoScalingGroupName,Value=org-staging-docker-asg-$INSTANCE_ID \
  --evaluation-periods 2 \
  --alarm-actions "arn:aws:autoscaling:us-east-1:341288657826:scalingPolicy:59a4e27a-0538-4edd-9fcb-dd9a6d9d5f77:autoScalingGroupName/org-staging-docker-asg-$INSTANCE_ID:policyName/org-staging-docker-sop"
```

and scale in:

``` bash
aws cloudwatch put-metric-alarm \
  --alarm-name org-staging-docker-$INSTANCE_ID-remove-capacity \
  --metric-name 'v1.travis.rabbitmq.queues.builds.docker.messages_ready' \
  --namespace Travis/org \
  --statistic Maximum \
  --period 120 \
  --threshold 1 \
  --comparison-operator LessThanOrEqualToThreshold \
  --dimensions Name=AutoScalingGroupName,Value=org-staging-docker-asg-$INSTANCE_ID \
  --evaluation-periods 2 \
  --alarm-actions "arn:aws:autoscaling:us-east-1:341288657826:scalingPolicy:ff543466-6f36-4d62-b41f-94601078b147:autoScalingGroupName/org-staging-docker-asg-$INSTANCE_ID:policyName/org-staging-docker-sip"
```

Because of the nature of the workload we typically run on our instances, we can't take advantage of plain autoscaling
policies that result in scale in/out with immediate instance termination.  Instead, we use lifecycle management events
to account for instance setup/teardown time.  Managing capacity in this way means more interactions between AWS and
pudding, as well as between pudding and the individual instances (via consul?).

Lifecycle hooks for both launching and terminating may be supported, e.g.:

``` bash
aws autoscaling put-lifecycle-hook \
  --auto-scaling-group-name org-staging-docker-asg-$INSTANCE_ID \
  --lifecycle-hook-name org-staging-docker-$INSTANCE_ID-lch-launching \
  --lifecycle-transition autoscaling:EC2_INSTANCE_LAUNCHING \
  --notification-target-arn arn:aws:sns:us-east-1:341288657826:pudding-test-topic \
  --role-arn arn:aws:iam::341288657826:role/pudding-sns-test

aws autoscaling put-lifecycle-hook \
  --auto-scaling-group-name org-staging-docker-asg-$INSTANCE_ID \
  --lifecycle-hook-name org-staging-docker-$INSTANCE_ID-lch-terminating \
  --lifecycle-transition autoscaling:EC2_INSTANCE_TERMINATING \
  --notification-target-arn arn:aws:sns:us-east-1:341288657826:pudding-test-topic \
  --role-arn arn:aws:iam::341288657826:role/pudding-sns-test
```

The actions taken for these lifecycle events are now in our control (as opposed to `shutdown -h now`).  Yay!

According to the AWS docs, this is the basic sequence for adding a lifecycle hook to an Auto Scaling Group:

1. Create a notification target. A target can be either an Amazon SQS queue or an Amazon SNS topic.
1. Create an IAM role. This role allows Auto Scaling to publish lifecycle notifications to the designated SQS queue or SNS
topic.
1. Create the lifecycle hook. You can create a hook that acts when instances launch or when instances terminate.
1. If necessary, record the lifecycle action heartbeat to keep the instance in a pending state.
1. Complete the lifecycle action.

The way this sequence can be applied to pudding might go something like this:

1. The SNS topic is expected to already exist, along with a confirmed subscription with an endpoint URL pointing back at
   pudding over https.  The topic ARN must be provided via env configuration a la `PUDDING_SNS_TOPIC_ARN`.
1. The IAM role is expected to already exist, and must be provided via env configuration a la `PUDDING_ROLE_ARN`.  The
   role must have a policy that allows for publishing to the sns topic.
1. Creation of the lifecycle hook(s) happens automatically during creation of the autoscaling group, with the
   asg-specific SNS topic being specified
1. Either have pudding repeatedly enqueue `RecordLifecycleActionHeartbeat` API calls, or perhaps set the
   `HeartbeatTimeout` higher than the build job timeout for the site/env.
1. During both instance launch and termination, the completion of the lifecycle will happen when the instance phones
   home to pudding and pudding then forwards the event as a `CompleteLifecycleAction` request.  In the case of the
launch event, this hook should probably fire when the instance is ready to begin consuming work and potentially wipe the
hook after the first execution so that subsequent restarts don't result in failed `CompleteLifecycleAction` requests.

## SNS Topic bits

Upon subscribing to an SNS Topic, the HTTP(S) URL will receive a subscription confirmation payload like this:

``` javascript
{
  "Type" : "SubscriptionConfirmation",
  "MessageId" : "98a3094e-c7e8-4d38-a730-939f361c6065",
  "Token" : "2336412f37fb687f5d51e6e241d638b114f4e9b52623c594ff666aff11609847fd78b02578f0a1aa8b6ff0ed1e5c37dfe94f118833bfc5b99b20240993dbe294721f4ebf79f904e692bcc4ef2d30af482bd4c1e7a4342d3483783da546e9d39da8315b1b28d6693fd54280be2df46a3befa6669a7a4c2661279cef2fa857d057",
  "TopicArn" : "arn:aws:sns:us-east-1:341288657826:pudding-test-topic",
  "Message" : "You have chosen to subscribe to the topic arn:aws:sns:us-east-1:341288657826:pudding-test-topic.\nTo confirm the subscription, visit the SubscribeURL included in this message.",
  "SubscribeURL" : "https://sns.us-east-1.amazonaws.com/?Action=ConfirmSubscription&TopicArn=arn:aws:sns:us-east-1:341288657826:pudding-test-topic&Token=2336412f37fb687f5d51e6e241d638b114f4e9b52623c594ff666aff11609847fd78b02578f0a1aa8b6ff0ed1e5c37dfe94f118833bfc5b99b20240993dbe294721f4ebf79f904e692bcc4ef2d30af482bd4c1e7a4342d3483783da546e9d39da8315b1b28d6693fd54280be2df46a3befa6669a7a4c2661279cef2fa857d057",
  "Timestamp" : "2014-12-22T20:29:28.282Z",
  "SignatureVersion" : "1",
  "Signature" : "oIcRPV7fIfrsGBElsVbWVOdXS7DeoDttUtGX386Hd2BRSWd8uzMKbF4F8GnrW/TKVmbXYu30/SlWAQKzhx7Ud2eMGqmVUZS96g2o2lkgyCl+VdkcfwYQ8TBGzmClVIEtsKV+map2yq6HIxxnQMNLGTxq/DT4NQGvqYaMet8mxq4roYM4lA/lNLZdLhYs9h8on5uxjAAw2WHQ/gUH2LxUx6N10CKSSV6lHQr+Ior0VLaAHNxCp2d0fLLJM3XvW0HUFZD5JEohq27/q5d37Uc3N7+DZ+fKrmurjkV721YwXgeHlo5a/lQ6WrEN4wpGznxFPBFlVtbczi/6HO+PsCpSqA==",
  "SigningCertURL" : "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-d6d679a1d18e95c2f9ffcf11f4f9e198.pem"
}
```

SNS Notifications have a body like this:

``` javascript
{
  "Type" : "Notification",
  "MessageId" : "375f381a-a143-50b5-8e61-7508234b4255",
  "TopicArn" : "arn:aws:sns:us-east-1:341288657826:pudding-test-topic",
  "Subject" : "Just Testing",
  "Message" : "This is a test eh",
  "Timestamp" : "2014-12-22T20:32:08.437Z",
  "SignatureVersion" : "1",
  "Signature" : "oxUggncdas6GcSzheXSRU9MZtvvFEGqd4IwGTG1ljj9CRWF/AxQ+/hS986bW4bGrh9ic5Z+uIUXRq/XfN34aFGMsLy9RSNgAwKoDe0e+g9OFWP3DrK+oe+Lr2HfwyRtS7J5YnHAeRkuuCIVkCRX+RgXLJvCfosSmgKGiYBToDakoEVsJyBh1MbuPCz33Czw974UdsWfCSzUhM0gOceQ6LbkHBUdfXcPH8wFVpoSoJZcnDIKqjTjRAhmYKdC85c2J1Jca35PY2gaPPDtiPtnoKxDMfJ4PTlrW2jVefaZjKBRj43o+aaWzBVNG1931OpjtMu6d5Lml/148bweB27am3A==",
  "SigningCertURL" : "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-d6d679a1d18e95c2f9ffcf11f4f9e198.pem",
  "UnsubscribeURL" : "https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=arn:aws:sns:us-east-1:341288657826:pudding-test-topic:8a210808-2c56-4f43-8411-bf23666b8625",
  "MessageAttributes" : {
    "AWS.SNS.MOBILE.MPNS.Type" : {"Type":"String","Value":"token"},
    "AWS.SNS.MOBILE.WNS.Type" : {"Type":"String","Value":"wns/badge"},
    "AWS.SNS.MOBILE.MPNS.NotificationClass" : {"Type":"String","Value":"realtime"}
  }
}
```

When a lifecycle hook is configured for an autoscaling group, a test notification is sent to the SNS topic with a
payload like this for each subscription (each lifecyle transition):

``` javascript
{
  "Type" : "Notification",
  "MessageId" : "3edbc59a-0358-5152-aa1a-88888b0e3347",
  "TopicArn" : "arn:aws:sns:us-east-1:341288657826:pudding-test-topic",
  "Subject" : "Auto Scaling: test notification for group \"org-staging-docker-asg-$INSTANCE_ID\"",
  "Message" : "{\"AutoScalingGroupName\":\"org-staging-docker-asg-$INSTANCE_ID\",\"Service\":\"AWS Auto Scaling\",\"Time\":\"2014-12-22T20:58:56.930Z\",\"AccountId\":\"341288657826\",\"Event\":\"autoscaling:TEST_NOTIFICATION\",\"RequestId\":\"585ad5cd-8a1d-11e4-b467-4194aad3947b\",\"AutoScalingGroupARN\":\"arn:aws:autoscaling:us-east-1:341288657826:autoScalingGroup:6b164a47-9782-493c-99d0-86e5ec3a8c1a:autoScalingGroupName/org-staging-docker-asg-$INSTANCE_ID\"}",
  "Timestamp" : "2014-12-22T20:59:02.057Z",
  "SignatureVersion" : "1",
  "Signature" : "wxMkfMRjZJWAK086ehDNZcLmQ4WPkO8V/biC7FjW5ok9SLH7jWbPHMyFYhBNfGEzOA2t2tVBuSUJDlzQ/jRjQQZqRx0Sgvtuvpwn9cHpRMJNWSxXkJP6Z8sD1I9S1NdNAADzEG02DV4zOZgkUVkItoGYrJw1DYO14/xQr9kcVDLNr2r6PJk1SLxR85Y+y72ZloKLshKYGdZlXqL5hv8DWa53hlzf1vEb+gZ2BTpjuFVxRaIbvsCconIXEDdOdSWOzW/9NzP46iDTAp79eBnENo+P5WYLCTUIX072eENZ+WnzuvCSMOI4uxB4/rqsj+BnirgTILztw6r5F7GMyqOLVg==",
  "SigningCertURL" : "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-d6d679a1d18e95c2f9ffcf11f4f9e198.pem",
  "UnsubscribeURL" : "https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=arn:aws:sns:us-east-1:341288657826:pudding-test-topic:8a210808-2c56-4f43-8411-bf23666b8625"
}
```

The `autoscaling:EC2_INSTANCE_TERMINATING` event results in a message like this:

``` javascript
{
  "Type" : "Notification",
  "MessageId" : "c87337ab-c19e-51f2-8a5d-7ab80071cc4b",
  "TopicArn" : "arn:aws:sns:us-east-1:341288657826:pudding-test-topic",
  "Subject" : "Auto Scaling:  Lifecycle action 'TERMINATING' for instance i-4c87e963 in progress.",
  "Message" : "{\"AutoScalingGroupName\":\"org-staging-docker-asg-$INSTANCE_ID\",\"Service\":\"AWS Auto Scaling\",\"Time\":\"2014-12-23T19:17:03.843Z\",\"AccountId\":\"341288657826\",\"LifecycleTransition\":\"autoscaling:EC2_INSTANCE_TERMINATING\",\"RequestId\":\"8fb86310-cc3f-45b6-9577-7997b4bfad0d\",\"LifecycleActionToken\":\"2f346e45-4866-4bf1-a752-f6eea23011c7\",\"EC2InstanceId\":\"i-4c87e963\",\"LifecycleHookName\":\"org-staging-docker-lch-$INSTANCE_ID-terminating\"}",
  "Timestamp" : "2014-12-23T19:17:03.874Z",
  "SignatureVersion" : "1",
  "Signature" : "S0oU0BB373Z1dm8d088j+5fD90A3ZD35xWsUrL93nRarX5P4dK+h0Yxsy79Ur1MeJQcdzYCbflHxyhywHuDWWDScxEHqOv7x5PRUPBCpz5BszTK52SEsXPd1LXS/dAZKU+zHBiV9/IJxEzzXgq4JlXrPUd4WCAr0zitZJi/1nhZWdoar41UATJcWh4xKszSmK5bV3CGd4OEs2CE4zdMktfpVGKxJ5qLVGnSqeO0jL6pTOh6hXQDighRTxU6ryrY0/n8ZlMkKxOs60x/hHsmPjRkITa6TRlfUt4y4f7H/K4OB+F/bM/svJvHJi7b+vQaHO0gIgLRSM1QglekcHQihSQ==",
  "SigningCertURL" : "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-d6d679a1d18e95c2f9ffcf11f4f9e198.pem",
  "UnsubscribeURL" : "https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=arn:aws:sns:us-east-1:341288657826:pudding-test-topic:8a210808-2c56-4f43-8411-bf23666b8625"
}
```

## Cycling out instances

Given that we're defining an autoscaling group from a template instance, the "cycling" or replacement process is a bit
involved.  The rough steps might be:

1. Get the current capacity of the existing autoscaling group
1. Create a new instance based on the latest or specified AMI
1. Once the instance has started, create a replacement autoscaling group from the instance id, setting the desired
   capacity to the existing autoscaling group's capacity
1. Create scaling policies, metric alarms, lifecycle hooks, etc. for the replacement autoscaling group that are copies
   of those assigned to the existing autoscaling group.
1. Set the desired capacity of the existing autoscaling group to 0.
1. Upon termination of all instances in the existing autoscaling group, delete the autoscaling group and all assigned
   resources.

Roughly the same process would apply to promoting a new AMI in a canary-style roll out, except that we would be
intentionally keeping more than one autoscaling group around for a given site-org-queue pool until the replacement is
complete.  Perhaps we should stick to full replacement (?)

## Problems:

### user-data init script lifespan

The `user-data` for the instances created by the worker manager service is currently doing an `#include` of an init
script URL that's intented to be short-lived.  If this user-data is going to be used within an auto-scaling context,
then we'll either have to know up front and give it a considerably longer expiry (or no expiry at all), or perhaps
remove expiry from init-scripts and their auths altogether.

### Name tags no longer unique

The `Name` tag for instances within an autoscaling group cannot (?) be based on the instance id, e.g.
`travis-org-staging-docker-abcd1234`.  One option is to do like the above `aws autoscaling create-auto-scaling-group`
invocation and assign a name that ends with the root instance id and `-asg`, but then individual instances within the
autoscaling group will not be unique.  This may require setting the system hostname dynamically during cloud init so
that it includes the instance id fetched from the metadata API.

## Putting it all together

When creating an autoscaling group in pudding, the required inputs are:

* an existing instance id OR an existing autoscaling group name *REQUIRED*
* an existing IAM role ARN for setting up SNS bits  *REQUIRED*
* site  *REQUIRED*
* env  *REQUIRED*
* queue  *REQUIRED*
* min size (default `0`)
* max size (default `1`)
* desired capacity (default `1`)
* scale out metric alarm spec, which is
  `{"namespace":"<namespace>","metric_name":"<metric-name>","statistic":"<statistic>","op":"<comparison-operator>","threshold":"<threshold>","period":"<period>","evaluation_periods":"<evaluation-periods>"}`
(default `{"namespace":"AWS/EC2","metric_name":"CPUUtilization","statistic":"Average","op":"GreaterThanOrEqualToThreshold","threshold":"95","period":"120","evaluation_periods":"2"}`)
* scale in metric alarm spec, which is
  `{"namespace":"<namespace>","metric_name":"<metric-name>","statistic":"<statistic>","op":"<comparison-operator>","threshold":"<threshold>","period":"<period>","evaluation_periods":"<evaluation-periods>"}`
(default `{"namespace":"AWS/EC2","metric_name":"CPUUtilization","statistic":"Average","op":"LessThanOrEqualToThreshold","threshold":"10","period":"120","evaluation_periods":"2"}`)

**Autoscaling group name**:`"{{.Site}}-{{.Env}}-{{.Queue}}-asg-{{.InstanceID}}"`.

Upon creation of the autoscaling group, the next step is to create scaling policies for scaling out and scaling in in
adjustments of 1.

**Scale out policy name**: `"{{.Site}}-{{.Env}}-{{.Queue}}-sop-{{.InstanceID}}"`

**Scale in policy name**: `"{{.Site}}-{{.Env}}-{{.Queue}}-sip-{{.InstanceID}}"`

The policy ARNs resulting from the creation of the scaling policies are then used to create metric alarms, the params
for which must be supplied at autoscaling group creation time.  For the purposes of scaling instances running
`travis-worker` and build env containers, it is unlikely we'll be able to use any of the builtin cloudwatch metrics, but
instead we would rely on a custom cloudwatch metric shipped from elsewhere such as rabbitmq messages ready.

**Scale out metric alarm name**: `"{{.Site}}-{{.Env}}-{{.Queue}}-soma-{{.InstanceID}}"`

**Scale in metric alarm name**: `"{{.Site}}-{{.Env}}-{{.Queue}}-sima-{{.InstanceID}}"`

Before being able to create lifecycle hooks for the autoscaling group, we'll have to create an SNS topic and subscribe
to it via HTTP(S).

**SNS topic name**: `"{{.Site}}-{{.Env}}-{{.Queue}}-topic-{{.InstanceID}}"`

Once we have the topic ARN, this is used to subscribe pudding to the topic, specifying a notification endpoint specific
to this autoscaling group, e.g. `https://$PUDDING_HOST/autoscaling-group-notifications/$AUTOSCALING_GROUP_NAME`.

As soon as this subscription is created, the expectation is that a subscription confirmation request will come to
pudding.  The request signature should be verified, then subscription confirmed.

At this point, the autoscaling group definition is complete.  The remaining work performed by pudding will be in the
form of responding to lifecycle hook notifications and custom internal events related to instance lifecycle management.
