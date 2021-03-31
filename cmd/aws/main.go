package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type cleaner interface {
	clean(ctx context.Context) error
}

type instanceCleaner struct {
	client      *ec2.Client
	instanceIDs []string
}

func (i *instanceCleaner) clean(ctx context.Context) error {
	log.Printf("terminating instances %v", i.instanceIDs)
	_, err := i.client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: i.instanceIDs,
	})
	return err
}

type controller struct {
	client  *ec2.Client
	cleanup []cleaner
}

func (c *controller) clean(ctx context.Context) error {
	log.Printf("initiating cleanup")
	var errs bool
	for i := len(c.cleanup) - 1; i >= 0; i-- {
		select {
		case <-ctx.Done():
			log.Println(ctx.Err())
			errs = true
			break
		default:
			if err := c.cleanup[i].clean(ctx); err != nil {
				log.Println(err)
				errs = true
			}
		}
	}
	if errs {
		return errors.New("errors encountered during cleanup, you will have to clean your account manually")
	}
	return nil
}

func (c *controller) runInstance(ctx context.Context) (run *ec2.RunInstancesOutput, err error) {
	var (
		ec2AMI = "ami-0ca5c3bd5a268e7db" // Ubuntu 20.04 LTS
	)
	run, err = c.client.RunInstances(ctx, &ec2.RunInstancesInput{
		MaxCount:     1,
		MinCount:     1,
		ImageId:      &ec2AMI,
		InstanceType: types.InstanceTypeT2Micro,
	})
	if err != nil {
		return nil, err
	}

	var instances []string
	for _, instance := range run.Instances {
		instances = append(instances, *instance.InstanceId)
	}
	c.cleanup = append(c.cleanup, &instanceCleaner{
		client:      c.client,
		instanceIDs: instances,
	})
	return run, nil
}

const (
	maxQueryAttempts = 10
	waitTime         = time.Second * 3
)

func (c *controller) waitForInstance(ctx context.Context, instanceID string) error {
	var attempts int
	log.Printf("waiting for instance %s to transition to running state", instanceID)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			describe, err := c.client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
				InstanceIds: []string{instanceID},
			})
			if err != nil {
				return err
			}
			if describe.Reservations[0].Instances[0].State.Name == types.InstanceStateNameRunning {
				return nil
			}
			if attempts >= maxQueryAttempts {
				return fmt.Errorf("instance not ready after %d seconds, aborting",
					maxQueryAttempts*waitTime)
			}
			log.Println("still waiting...")
			attempts++
		}
	}
}

func (c *controller) terminateInstances(ctx context.Context, instanceIds []string) error {
	_, err := c.client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: instanceIds,
	})
	return err
}

func (c *controller) listInstances(ctx context.Context) (reservations []types.Reservation, err error) {
	paginator := ec2.NewDescribeInstancesPaginator(c.client, &ec2.DescribeInstancesInput{})
	for paginator.HasMorePages() {
		output, err := paginator.NextPage(ctx)
		if err != nil {
			return reservations, err
		}
		reservations = append(reservations, output.Reservations...)
	}
	return
}

func main() {
	cfg, err := config.LoadDefaultConfig(context.Background(), config.WithRegion("us-west-2"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	c := &controller{
		client: ec2.NewFromConfig(cfg),
	}
	_ = c

	// output(c.listInstances(context.Background()))

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		signal.Stop(sig)
		cancel()
		if err := c.clean(context.Background()); err != nil {
			log.Println(err)
			os.Exit(1)
		}
	}()

	go func() {
		select {
		case <-sig:
			cancel()
		case <-ctx.Done():
		}
		<-sig
		log.Println("ungraceful exit")
		os.Exit(1)
	}()

	instance, err := c.runInstance(ctx)
	if err != nil {
		panic(err)
	}
	if c.waitForInstance(ctx, *instance.Instances[0].InstanceId); err != nil {
		panic(err)
	}

	// err = c.terminateInstances(context.Background(), []string{"i-08f92cd41c59f4360"})
}

func output(i interface{}, err error) {
	if err != nil {
		fmt.Println(err)
	} else {
		byt, err := json.MarshalIndent(i, "", "  ")
		if err != nil {
			panic(err)
		}
		fmt.Println(string(byt))
	}
}
