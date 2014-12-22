# Managing autoscaling bits

A new autoscaling group may be defined by specifying only the name of the autoscaling group and an existing instance id.
Without specifying the tags, only the autoscaling group name tag will be applied, though, so instead tags should be
passed at autoscaling group creation time, e.g.:

``` bash
aws autoscaling create-auto-scaling-group \
  --instance-id i-80fd91af \
  --tags \
    Key=role,Value=worker \
    Key=queue,Value=docker \
    Key=site,Value=org \
    Key=env,Value=staging \
    Key=Name,Value=org-staging-docker-asg \
  --min-size 1 \
  --max-size 3 \
  --desired-capacity 1 \
  --auto-scaling-group-name org-staging-docker-asg
```

Because of the nature of the workload we typically run on our instances, we can't take advantage of plain autoscaling
policies that result in scale in/out with immediate instance termination.  Instead, we use lifecycle management events
to account for instance setup/teardown time.  Managing capacity in this way means more interactions between AWS and
pudding, as well as between pudding and the individual instances (via consul?).

Lifecycle hooks for both launching and terminating may be supported, e.g.:

``` bash
aws autoscaling put-lifecycle-hook \
  --auto-scaling-group-name org-staging-docker-asg \
  --lifecycle-hook-name org-staging-docker-lch-launching \
  --lifecycle-transition autoscaling:EC2_INSTANCE_LAUNCHING \
  --notification-target-arn arn:aws:sns:us-east-1:341288657826:pudding-test-topic \
  --role-arn arn:aws:iam::341288657826:role/pudding-sns-test

aws autoscaling put-lifecycle-hook \
  --auto-scaling-group-name org-staging-docker-asg \
  --lifecycle-hook-name org-staging-docker-lch-terminating \
  --lifecycle-transition autoscaling:EC2_INSTANCE_TERMINATING \
  --notification-target-arn arn:aws:sns:us-east-1:341288657826:pudding-test-topic \
  --role-arn arn:aws:iam::341288657826:role/pudding-sns-test
```

The actions taken for these lifecycle events are now in our control (as opposed to `shutdown -h now`).  Yay!  Part of
the reason why pudding exists is because we want AWS credentials and awareness to be limited so that we can leave open
the possibility of plugging in different backends in the future.

According to the AWS docs, these is the basic sequence for adding a lifecycle hook to an Auto Scaling Group:

1. Create a notification target. A target can be either an Amazon SQS queue or an Amazon SNS topic.
1. Create an IAM role. This role allows Auto Scaling to publish lifecycle notifications to the designated SQS queue or SNS
topic.
1. Create the lifecycle hook. You can create a hook that acts when instances launch or when instances terminate.
1. If necessary, record the lifecycle action heartbeat to keep the instance in a pending state.
1. Complete the lifecycle action.

The way this sequence can be applied to pudding might go something like this:

1. Upon fulfilling a request to create an autoscaling group, pudding will also:
    * create an SNS topic specific to the autoscaling group
    * subscribe to the SNS topic pointing back at pudding
    * confirm the SNS subscription
1. The IAM role is expected to already exist, and must be provided via env configuration a la `PUDDING_ROLE_ARN`
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
  "Subject" : "Auto Scaling: test notification for group \"org-staging-docker-asg\"",
  "Message" : "{\"AutoScalingGroupName\":\"org-staging-docker-asg\",\"Service\":\"AWS Auto Scaling\",\"Time\":\"2014-12-22T20:58:56.930Z\",\"AccountId\":\"341288657826\",\"Event\":\"autoscaling:TEST_NOTIFICATION\",\"RequestId\":\"585ad5cd-8a1d-11e4-b467-4194aad3947b\",\"AutoScalingGroupARN\":\"arn:aws:autoscaling:us-east-1:341288657826:autoScalingGroup:6b164a47-9782-493c-99d0-86e5ec3a8c1a:autoScalingGroupName/org-staging-docker-asg\"}",
  "Timestamp" : "2014-12-22T20:59:02.057Z",
  "SignatureVersion" : "1",
  "Signature" : "wxMkfMRjZJWAK086ehDNZcLmQ4WPkO8V/biC7FjW5ok9SLH7jWbPHMyFYhBNfGEzOA2t2tVBuSUJDlzQ/jRjQQZqRx0Sgvtuvpwn9cHpRMJNWSxXkJP6Z8sD1I9S1NdNAADzEG02DV4zOZgkUVkItoGYrJw1DYO14/xQr9kcVDLNr2r6PJk1SLxR85Y+y72ZloKLshKYGdZlXqL5hv8DWa53hlzf1vEb+gZ2BTpjuFVxRaIbvsCconIXEDdOdSWOzW/9NzP46iDTAp79eBnENo+P5WYLCTUIX072eENZ+WnzuvCSMOI4uxB4/rqsj+BnirgTILztw6r5F7GMyqOLVg==",
  "SigningCertURL" : "https://sns.us-east-1.amazonaws.com/SimpleNotificationService-d6d679a1d18e95c2f9ffcf11f4f9e198.pem",
  "UnsubscribeURL" : "https://sns.us-east-1.amazonaws.com/?Action=Unsubscribe&SubscriptionArn=arn:aws:sns:us-east-1:341288657826:pudding-test-topic:8a210808-2c56-4f43-8411-bf23666b8625"
}
```

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
