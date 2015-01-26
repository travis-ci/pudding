package workers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"

	"github.com/Sirupsen/logrus"
	"github.com/garyburd/redigo/redis"
	"github.com/goamz/goamz/autoscaling"
	"github.com/goamz/goamz/cloudwatch"
	"github.com/goamz/goamz/ec2"
	"github.com/jrallison/go-workers"
	"github.com/travis-ci/pudding/lib"
)

func init() {
	defaultQueueFuncs["autoscaling-group-builds"] = autoscalingGroupBuildsMain
}

func autoscalingGroupBuildsMain(cfg *internalConfig, msg *workers.Msg) {
	buildPayloadJSON := []byte(msg.OriginalJson())
	buildPayload := &lib.AutoscalingGroupBuildPayload{
		Args: []*lib.AutoscalingGroupBuild{},
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
	n      []lib.Notifier
	jid    string
	cfg    *internalConfig
	ec2    *ec2.EC2
	as     *autoscaling.AutoScaling
	cw     *cloudwatch.CloudWatch
	b      *lib.AutoscalingGroupBuild
	name   string
	sopARN string
	sipARN string
}

func newAutoscalingGroupBuilderWorker(b *lib.AutoscalingGroupBuild, cfg *internalConfig, jid string, redisConn redis.Conn) (*autoscalingGroupBuilderWorker, error) {
	notifier := lib.NewSlackNotifier(cfg.SlackHookPath, cfg.SlackUsername, cfg.SlackIcon)

	cw, err := cloudwatch.NewCloudWatch(cfg.AWSAuth, cfg.AWSRegion.CloudWatchServicepoint)
	if err != nil {
		return nil, err
	}

	return &autoscalingGroupBuilderWorker{
		rc:  redisConn,
		jid: jid,
		cfg: cfg,
		n:   []lib.Notifier{notifier},
		b:   b,
		ec2: ec2.New(cfg.AWSAuth, cfg.AWSRegion),
		as:  autoscaling.New(cfg.AWSAuth, cfg.AWSRegion),
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

func (asgbw *autoscalingGroupBuilderWorker) createAutoscalingGroup() (*autoscaling.CreateAutoScalingGroupParams, error) {
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

	tags := []autoscaling.Tag{
		autoscaling.Tag{
			Key: "role", Value: b.Role, PropagateAtLaunch: true,
		},
		autoscaling.Tag{
			Key: "queue", Value: b.Queue, PropagateAtLaunch: true,
		},
		autoscaling.Tag{
			Key: "site", Value: b.Site, PropagateAtLaunch: true,
		},
		autoscaling.Tag{
			Key: "env", Value: b.Env, PropagateAtLaunch: true,
		},
		autoscaling.Tag{
			Key: "Name", Value: asgbw.name, PropagateAtLaunch: true,
		},
	}

	asg := &autoscaling.CreateAutoScalingGroupParams{
		AutoScalingGroupName: asgbw.name,
		InstanceId:           b.InstanceID,
		MinSize:              b.MinSize,
		MaxSize:              b.MaxSize,
		DesiredCapacity:      b.DesiredCapacity,
		Tags:                 tags,
	}

	log.WithFields(logrus.Fields{
		"jid": asgbw.jid,
		"asg": fmt.Sprintf("%#v", asg),
	}).Debug("creating autoscaling group")

	_, err = asgbw.as.CreateAutoScalingGroup(asg)
	return asg, err
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleOutPolicy() (string, error) {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating scale out policy")

	sop := &autoscaling.PutScalingPolicyParams{
		PolicyName:           fmt.Sprintf("%s-sop", asgbw.name),
		AutoScalingGroupName: asgbw.name,
		AdjustmentType:       "ChangeInCapacity",
		Cooldown:             asgbw.b.ScaleOutCooldown,
		ScalingAdjustment:    asgbw.b.ScaleOutAdjustment,
	}

	resp, err := asgbw.as.PutScalingPolicy(sop)
	if err != nil {
		return "", err
	}

	return resp.PolicyARN, nil
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleInPolicy() (string, error) {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating scale in policy")

	sip := &autoscaling.PutScalingPolicyParams{
		PolicyName:           fmt.Sprintf("%s-sip", asgbw.name),
		AutoScalingGroupName: asgbw.name,
		AdjustmentType:       "ChangeInCapacity",
		Cooldown:             asgbw.b.ScaleInCooldown,
		ScalingAdjustment:    asgbw.b.ScaleInAdjustment,
	}

	resp, err := asgbw.as.PutScalingPolicy(sip)
	if err != nil {
		return "", err
	}

	return resp.PolicyARN, nil
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleOutMetricAlarm() error {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating scale out metric alarm")

	ma := &cloudwatch.MetricAlarm{
		AlarmName:          fmt.Sprintf("%s-add-capacity", asgbw.name),
		MetricName:         asgbw.b.ScaleOutMetricName,
		Namespace:          asgbw.b.ScaleOutMetricNamespace,
		Statistic:          asgbw.b.ScaleOutMetricStatistic,
		Period:             asgbw.b.ScaleOutMetricPeriod,
		Threshold:          asgbw.b.ScaleOutMetricThreshold,
		ComparisonOperator: asgbw.b.ScaleOutMetricComparisonOperator,
		EvaluationPeriods:  asgbw.b.ScaleOutMetricEvaluationPeriods,
		AlarmActions: []cloudwatch.AlarmAction{
			cloudwatch.AlarmAction{
				ARN: asgbw.sopARN,
			},
		},
		Dimensions: []cloudwatch.Dimension{
			cloudwatch.Dimension{
				Name:  "AutoScalingGroupName",
				Value: asgbw.name,
			},
		},
	}

	_, err := asgbw.cw.PutMetricAlarm(ma)
	return err
}

func (asgbw *autoscalingGroupBuilderWorker) createScaleInMetricAlarm() error {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating scale in metric alarm")

	ma := &cloudwatch.MetricAlarm{
		AlarmName:          fmt.Sprintf("%s-remove-capacity", asgbw.name),
		MetricName:         asgbw.b.ScaleInMetricName,
		Namespace:          asgbw.b.ScaleInMetricNamespace,
		Statistic:          asgbw.b.ScaleInMetricStatistic,
		Period:             asgbw.b.ScaleInMetricPeriod,
		Threshold:          asgbw.b.ScaleInMetricThreshold,
		ComparisonOperator: asgbw.b.ScaleInMetricComparisonOperator,
		EvaluationPeriods:  asgbw.b.ScaleInMetricEvaluationPeriods,
		AlarmActions: []cloudwatch.AlarmAction{
			cloudwatch.AlarmAction{
				ARN: asgbw.sipARN,
			},
		},
		Dimensions: []cloudwatch.Dimension{
			cloudwatch.Dimension{
				Name:  "AutoScalingGroupName",
				Value: asgbw.name,
			},
		},
	}

	_, err := asgbw.cw.PutMetricAlarm(ma)
	return err
}

func (asgbw *autoscalingGroupBuilderWorker) createLaunchingLifecycleHook() error {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating launching lifecycle hook")

	llch := &autoscaling.PutLifecycleHookParams{
		AutoScalingGroupName:  asgbw.name,
		LifecycleHookName:     fmt.Sprintf("%s-lch-launching", asgbw.name),
		LifecycleTransition:   "autoscaling:EC2_INSTANCE_LAUNCHING",
		NotificationTargetARN: asgbw.b.TopicARN,
		RoleARN:               asgbw.b.RoleARN,
	}

	_, err := asgbw.as.PutLifecycleHook(llch)
	return err
}

func (asgbw *autoscalingGroupBuilderWorker) createTerminatingLifecycleHook() error {
	log.WithFields(logrus.Fields{
		"jid":  asgbw.jid,
		"name": asgbw.name,
	}).Debug("creating terminating lifecycle hook")

	tlch := &autoscaling.PutLifecycleHookParams{
		AutoScalingGroupName:  asgbw.name,
		LifecycleHookName:     fmt.Sprintf("%s-lch-terminating", asgbw.name),
		LifecycleTransition:   "autoscaling:EC2_INSTANCE_TERMINATING",
		NotificationTargetARN: asgbw.b.TopicARN,
		RoleARN:               asgbw.b.RoleARN,
	}

	_, err := asgbw.as.PutLifecycleHook(tlch)
	return err
}
