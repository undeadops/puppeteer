package main

import (
	"net/url"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws/awserr"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	rabbithole "github.com/michaelklishin/rabbit-hole"
	"github.com/prometheus/client_golang/prometheus"
)

func scaleRunner() {
	//Autoscale Group Struct
	asg := cfg.ASG
	for _, asg := range asg {
		switch method := asg.Method; method {
		case "scale":
			Info.Println("scale method firing")
			scaleCluster(asg)
		default:
			Info.Println("Nothing to do")
		}
	}
}

var (
	scaleUpBy   = 4
	scaleDownBy = 2
	asgDesired  int
	asgMin      int
	asgMax      int
)

func scaleCluster(asg ASG) {
	for {
		asgDesired, asgMin, asgMax = getAutoScaleDesired(asg)
		// Report current count to prometheus exporter
		promASGcount.With(prometheus.Labels{"name": asg.AsGroupName}).Set(float64(asgDesired))

		rmqc := amqScaleConnection(asg)
		// Need to handle timeouts...
		mq, err := rmqc.GetQueue("/", asg.Queue)
		if err != nil {
			Warning.Printf("Error : %s", err)
		}
		Info.Printf("%s = %d\n", asg.Queue, mq.Messages)

		if mq.Messages < asg.Watermark {
			scaleDown(asg)
		} else if mq.Messages >= asg.Threshold {
			scaleUp(asg)
		} else {
			Info.Printf("Queue within normal range, doing nothing\n")
		}

		Info.Printf("ASG: %s, Current: %d, Min: %d, Max: %d\n", asg.AsGroupName, asgDesired, asgMin, asgMax)
		time.Sleep(95 * time.Second)
	}
}

func scaleUp(asg ASG) {
	var d int
	d = asgDesired + scaleUpBy
	if d >= asgMax && asgDesired < asgMax {
		scaleASG(asg, asgMax)
	} else if d < asgMax {
		scaleASG(asg, d)
	} else {
		Info.Printf("Workers Already Scaled up to Max\n")
	}
}

func scaleDown(asg ASG) {
	var d int
	d = asgDesired - scaleDownBy
	if d < asgMin && asgDesired > asgMin {
		scaleASG(asg, asgMin)
	} else if d >= asgMin {
		scaleASG(asg, d)
	} else {
		Info.Printf("Workers Already Scaled to Min\n")
	}
}

func amqScaleConnection(q ASG) *rabbithole.Client {
	amqURL := os.Getenv(q.AmqHost)
	if amqURL == "" {
		Warning.Panic("Missing AMQ Environment Variable")
	}
	u, err := url.Parse(amqURL)
	if err != nil {
		Warning.Panic(err)
	}

	password, _ := u.User.Password()
	rmqc, _ := rabbithole.NewClient(u.Scheme+"://"+u.Host, u.User.Username(), password)

	return rmqc
}

func getAutoScaleDesired(asg ASG) (int, int, int) {
	var desired int
	var min int
	var max int

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		Warning.Panic("failed to load config, " + err.Error())
	}
	cfg.Region = asg.AWSRegion

	svc := autoscaling.New(cfg)
	input := &autoscaling.DescribeAutoScalingGroupsInput{
		AutoScalingGroupNames: []string{
			asg.AsGroupName,
		},
	}

	req := svc.DescribeAutoScalingGroupsRequest(input)
	result, err := req.Send()
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case autoscaling.ErrCodeInvalidNextToken:
				Warning.Println(autoscaling.ErrCodeInvalidNextToken, aerr.Error())
			case autoscaling.ErrCodeResourceContentionFault:
				Warning.Println(autoscaling.ErrCodeResourceContentionFault, aerr.Error())
			default:
				Warning.Println(aerr.Error())
			}
		} else {
			// Print the error, cast err to awserr.Error to get the Code and
			// Message from an error.
			Warning.Println(err.Error())
		}
	}

	for _, g := range result.AutoScalingGroups {
		desired = int(*g.DesiredCapacity)
		min = int(*g.MinSize)
		max = int(*g.MaxSize)
	}
	return desired, min, max
}

func scaleASG(asg ASG, desired int) {
	Info.Printf("I will scale ASG to: %d\n", desired)

	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		Warning.Panic("failed to load config, " + err.Error())
	}
	cfg.Region = asg.AWSRegion

	svc := autoscaling.New(cfg)

	var coolDown = true
	if asg.DisableCoolDown {
		coolDown = false
	}

	desiredCapacity := int64(desired)

	input := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: &asg.AsGroupName,
		DesiredCapacity:      &desiredCapacity,
		HonorCooldown:        &coolDown,
	}

	if asg.Enabled {
		promASGscale.With(prometheus.Labels{"name": asg.AsGroupName}).Inc()
		req := svc.SetDesiredCapacityRequest(input)
		resp, err := req.Send()
		if err != nil {
			Info.Println("Scaling Failed, Cooldown window may be active")
			Info.Println(resp)
		}

	} else {
		Info.Println("Scaling is disabled: envvar Enabled: True to enable")
	}
}
