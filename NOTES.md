# Managing autoscaling bits

A new autoscaling group may be defined by specifying only the name of the autoscaling group and an existing instance id.
Without specifying the tags, only the autoscaling group name tag will be applied, though, so instead tags should be
passed at autoscaling group creation time, e.g.:

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

Because of the nature of the workload we typically run on our instances, we can't take advantage of plain autoscaling
policies that result in scale in/out with immediate instance termination.  Instead, we use lifecycle management events
to account for instance setup/teardown time.  Managing capacity in this way means more interactions between AWS and
pudding, as well as between pudding and the individual instances (via consul?).

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
