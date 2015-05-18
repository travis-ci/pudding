package workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/Sirupsen/logrus"
	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/autoscaling"
	"github.com/awslabs/aws-sdk-go/service/cloudwatch"
	"github.com/awslabs/aws-sdk-go/service/ec2"
	"github.com/garyburd/redigo/redis"
	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding"
)

func init() {
	defaultQueueFuncs["autoscaling-group-builds"] = autoscalingGroupBuildsMain
}

func autoscalingGroupBuildsMain(cfg *internalConfig, msg *workers.Msg) {
	buildPayloadJSON := []byte(msg.OriginalJson())
	buildPayload := &pudding.AutoscalingGroupBuildPayload{
		Args: []*pudding.AutoscalingGroupBuild{},
	}

	err := json.Unmarshal(buildPayloadJSON, buildPayload)
	if err != nil {
		log.WithField("err", err).Panic("failed to deserialize message")
	}

	b := buildPayload.AutoscalingGroupBuild()
	b.Hydrate()

	w, err := newAutoscalingGroupBuilderWorker(b, cfg, msg.Jid(), workers.Config.Pool.Get())
	if err != nil {
		log.WithField("err", err).Panic("autoscaling group build worker creation failed")
	}

	err = w.Build()
	if err != nil {
		log.WithField("err", err).Panic("autoscaling group build failed")
	}
}

type autoscalingGroupBuilderWorker struct {
	rc     redis.Conn
	n      []pudding.Notifier
	jid    string
	cfg    *internalConfig
	ec2    *ec2.EC2
	as     *autoscaling.AutoScaling
	cw     *cloudwatch.CloudWatch
	b      *pudding.AutoscalingGroupBuild
	name   string
	sopARN *string
	sipARN *string
}

func newAutoscalingGroupBuilderWorker(b *pudding.AutoscalingGroupBuild, cfg *internalConfig, jid string, redisConn redis.Conn) (*autoscalingGroupBuilderWorker, error) {
	notifier := pudding.NewSlackNotifier(cfg.SlackHookPath, cfg.SlackUsername, cfg.SlackIcon)

	cw := cloudwatch.New(cfg.AWSConfig)

	return &autoscalingGroupBuilderWorker{
		rc:  redisConn,
		jid: jid,
		cfg: cfg,
		n:   []pudding.Notifier{notifier},
		b:   b,
		ec2: ec2.New(cfg.AWSConfig),
		as:  autoscaling.New(cfg.AWSConfig),
		cw:  cw,
	}, nil
}

func (asgbw *autoscalingGroupBuilderWorker) Build() error {
	asg, err := asgbw.createAutoscalingGroup()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err": err,
			"jid": asgbw.jid,
		}).Error("failed to create autoscaling group")
		return err
	}

	sopARN, err := asgbw.createScaleOutPolicy()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.AutoScalingGroupName,
			"jid":  asgbw.jid,
		}).Error("failed to create scale out policy")
		return err
	}

	asgbw.sopARN = sopARN

	sipARN, err := asgbw.createScaleInPolicy()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.AutoScalingGroupName,
			"jid":  asgbw.jid,
		}).Error("failed to create scale in policy")
		return err
	}

	asgbw.sipARN = sipARN

	err = asgbw.createScaleOutMetricAlarm()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.AutoScalingGroupName,
			"jid":  asgbw.jid,
		}).Error("failed to create scale out metric alarm")
		return err
	}

	err = asgbw.createScaleInMetricAlarm()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.AutoScalingGroupName,
			"jid":  asgbw.jid,
		}).Error("failed to create scale in metric alarm")
		return err
	}

	err = asgbw.createLaunchingLifecycleHook()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.AutoScalingGroupName,
			"jid":  asgbw.jid,
		}).Error("failed to create launching lifecycle hook")
		return err
	}

	err = asgbw.createTerminatingLifecycleHook()
	if err != nil {
		log.WithFields(logrus.Fields{
			"err":  err,
			"name": asg.AutoScalingGroupName,
			"jid":  asgbw.jid,
		}).Error("failed to create terminating lifecycle hook")
		return err
	}

	log.WithField("jid", asgbw.jid).Debug("all done")
	return nil
}

func (asgbw *autoscalingGroupBuilderWorker) createAutoscalingGroup() (*autoscaling.CreateAutoScalingGroupInput, error) {
	b := asgbw.b

	nameTmpl, err := template.New(fmt.Sprintf("name-template-%s", asgbw.jid)).Parse(b.NameTemplate)
	if err != nil {
		return nil, err
	}

	var nameBuf bytes.Buffer
	err = nameTmpl.Execute(&nameBuf, b)
	if err != nil {
		return nil, err
	}

	asgbw.name = nameBuf.String()

	tags := []*autoscaling.Tag{
		&autoscaling.Tag{
			Key: aws.String("role"), Value: aws.String(b.Role), PropagateAtLaunch: aws.Boolean(true),
		},
		&autoscaling.Tag{
			Key: aws.String("queue"), Value: aws.String(b.Queue), PropagateAtLaunch: aws.Boolean(true),
		},
		&autoscaling.Tag{
			Key: aws.String("site"), Value: aws.String(b.Site), PropagateAtLaunch: aws.Boolean(true),
		},
		&autoscaling.Tag{
			Key: aws.String("env"), Value: aws.String(b.Env), PropagateAtLaunch: aws.Boolean(true),
		},
		&autoscaling.Tag{
			Key: aws.String("Name"), Value: aws.String(asgbw.name), PropagateAtLaunch: aws.Boolean(true),
		},
	}

	asg := &autoscaling.CreateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(asgbw.name),
		InstanceID:           aws.String(b.InstanceID),
		MinSize:              aws.Long(int64(b.MinSize)),
		MaxSize:              aws.Long(int64(b.MaxSize)),
		DesiredCapacity:      aws.Long(int64(b.DesiredCapacity)),
		DefaultCooldown:      aws.Long(int64(b.DefaultCooldown)),
		Tags:                 tags,
	}

	log.WithFields(logrus.Fields{
		"jid": asgbw.jid,
		"asg": fmt.Sprintf("%#v", asg),
	}).Debug("creating autoscaling group")

	_, err = asgbw.as.CreateAutoScalingGroup(asg)
	return asg, err
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleOutPolicy() (*string, error) {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating scale out policy")

	sop := &autoscaling.PutScalingPolicyInput{
		PolicyName:           aws.String(fmt.Sprintf("%s-sop", asgbw.name)),
		AutoScalingGroupName: aws.String(asgbw.name),
		AdjustmentType:       aws.String("ChangeInCapacity"),
		Cooldown:             aws.Long(int64(asgbw.b.ScaleOutCooldown)),
		ScalingAdjustment:    aws.Long(int64(asgbw.b.ScaleOutAdjustment)),
	}

	resp, err := asgbw.as.PutScalingPolicy(sop)
	if err != nil {
		return nil, err
	}

	return resp.PolicyARN, nil
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleInPolicy() (*string, error) {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating scale in policy")

	sip := &autoscaling.PutScalingPolicyInput{
		PolicyName:           aws.String(fmt.Sprintf("%s-sip", asgbw.name)),
		AutoScalingGroupName: aws.String(asgbw.name),
		AdjustmentType:       aws.String("ChangeInCapacity"),
		Cooldown:             aws.Long(int64(asgbw.b.ScaleInCooldown)),
		ScalingAdjustment:    aws.Long(int64(asgbw.b.ScaleInAdjustment)),
	}

	resp, err := asgbw.as.PutScalingPolicy(sip)
	if err != nil {
		return nil, err
	}

	return resp.PolicyARN, nil
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleOutMetricAlarm() error {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating scale out metric alarm")

	input := &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(fmt.Sprintf("%s-add-capacity", asgbw.name)),
		MetricName:         aws.String(asgbw.b.ScaleOutMetricName),
		Namespace:          aws.String(asgbw.b.ScaleOutMetricNamespace),
		Statistic:          aws.String(asgbw.b.ScaleOutMetricStatistic),
		Period:             aws.Long(int64(asgbw.b.ScaleOutMetricPeriod)),
		Threshold:          aws.Double(asgbw.b.ScaleOutMetricThreshold),
		ComparisonOperator: aws.String(asgbw.b.ScaleOutMetricComparisonOperator),
		EvaluationPeriods:  aws.Long(int64(asgbw.b.ScaleOutMetricEvaluationPeriods)),
		AlarmActions: []*string{
			asgbw.sopARN,
		},
	}

	_, err := asgbw.cw.PutMetricAlarm(input)
	return err
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleInMetricAlarm() error {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating scale in metric alarm")

	input := &cloudwatch.PutMetricAlarmInput{
		AlarmName:          aws.String(fmt.Sprintf("%s-remove-capacity", asgbw.name)),
		MetricName:         aws.String(asgbw.b.ScaleInMetricName),
		Namespace:          aws.String(asgbw.b.ScaleInMetricNamespace),
		Statistic:          aws.String(asgbw.b.ScaleInMetricStatistic),
		Period:             aws.Long(int64(asgbw.b.ScaleInMetricPeriod)),
		Threshold:          aws.Double(asgbw.b.ScaleInMetricThreshold),
		ComparisonOperator: aws.String(asgbw.b.ScaleInMetricComparisonOperator),
		EvaluationPeriods:  aws.Long(int64(asgbw.b.ScaleInMetricEvaluationPeriods)),
		AlarmActions: []*string{
			asgbw.sipARN,
		},
	}

	_, err := asgbw.cw.PutMetricAlarm(input)
	return err
}

func (asgbw *autoscalingGroupBuilderWorker) createLaunchingLifecycleHook() error {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating launching lifecycle hook")

	llch := &autoscaling.PutLifecycleHookInput{
		AutoScalingGroupName:  aws.String(asgbw.name),
		DefaultResult:         aws.String(asgbw.b.LifecycleDefaultResult),
		HeartbeatTimeout:      aws.Long(int64(asgbw.b.LifecycleHeartbeatTimeout)),
		LifecycleHookName:     aws.String(fmt.Sprintf("%s-lch-launching", asgbw.name)),
		LifecycleTransition:   aws.String("autoscaling:EC2_INSTANCE_LAUNCHING"),
		NotificationTargetARN: aws.String(asgbw.b.TopicARN),
		RoleARN:               aws.String(asgbw.b.RoleARN),
	}

	_, err := asgbw.as.PutLifecycleHook(llch)
	return err
}

func (asgbw *autoscalingGroupBuilderWorker) createTerminatingLifecycleHook() error {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating terminating lifecycle hook")

	tlch := &autoscaling.PutLifecycleHookInput{
		AutoScalingGroupName:  aws.String(asgbw.name),
		DefaultResult:         aws.String(asgbw.b.LifecycleDefaultResult),
		HeartbeatTimeout:      aws.Long(int64(asgbw.b.LifecycleHeartbeatTimeout)),
		LifecycleHookName:     aws.String(fmt.Sprintf("%s-lch-terminating", asgbw.name)),
		LifecycleTransition:   aws.String("autoscaling:EC2_INSTANCE_TERMINATING"),
		NotificationTargetARN: aws.String(asgbw.b.TopicARN),
		RoleARN:               aws.String(asgbw.b.RoleARN),
	}

	_, err := asgbw.as.PutLifecycleHook(tlch)
	return err
}
