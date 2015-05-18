package pudding

import (
	"fmt"
	"sort"

	"github.com/awslabs/aws-sdk-go/aws"
	"github.com/awslabs/aws-sdk-go/service/ec2"
)

var (
	errNoLatestImage = fmt.Errorf("no latest image available matching filter")
	activeFilter     = &ec2.Filter{
		Name:   aws.String("tag:key"),
		Values: []*string{aws.String("active")},
	}
)

// ResolveAMI attempts to get an ec2.Image by id, falling back to
// fetching the most recently provisioned worker ami via
// FetchLatestWorkerAMI
func ResolveAMI(conn *ec2.EC2, ID string, f *ec2.Filter) (*ec2.Image, error) {
	if ID != "" {
		resp, err := conn.DescribeImages(&ec2.DescribeImagesInput{
			DryRun:   aws.Boolean(false),
			ImageIDs: []*string{aws.String(ID)},
			Owners:   []*string{aws.String("self")},
		})

		if awserr := aws.Error(err); awserr != nil {
			return nil, err
		}
		for _, img := range resp.Images {
			if *img.ImageID == ID {
				return img, nil
			}
		}
	}

	return FetchLatestAMIWithFilter(conn, f)
}

// FetchLatestAMIWithFilter looks up all images matching the given
// filter (with `tag:active=true` added), then sorts by the image
// name which is assumed to contain a timestamp, then returns the
// most recent image.
func FetchLatestAMIWithFilter(conn *ec2.EC2, f *ec2.Filter) (*ec2.Image, error) {
	allImages, err := conn.DescribeImages(&ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{f, activeFilter},
		Owners:  []*string{aws.String("self")},
	})
	if awserr := aws.Error(err); awserr != nil {
		return nil, err
	}

	if len(allImages.Images) == 0 {
		return nil, errNoLatestImage
	}

	imgNames := []string{}
	imgMap := map[string]*ec2.Image{}

	for _, img := range allImages.Images {
		imgNames = append(imgNames, *img.Name)
		imgMap[*img.Name] = img
	}

	sort.Strings(imgNames)
	img := imgMap[imgNames[len(imgNames)-1]]
	return img, nil
}

// GetInstancesWithFilter fetches all instances that match the
// given filter
func GetInstancesWithFilter(conn *ec2.EC2, f *ec2.Filter) (map[string]*ec2.Instance, error) {
	resp, err := conn.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{f},
	})

	if err != nil {
		return nil, err
	}

	instances := map[string]*ec2.Instance{}

	for _, res := range resp.Reservations {
		for _, inst := range res.Instances {
			instances[*inst.InstanceID] = inst
		}
	}

	return instances, nil
}

// GetImagesWithFilter fetches all images that match the
// given filter
func GetImagesWithFilter(conn *ec2.EC2, f *ec2.Filter) (map[string]*ec2.Image, error) {
	resp, err := conn.DescribeImages(&ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{f},
	})

	if err != nil {
		return nil, err
	}

	images := map[string]*ec2.Image{}

	for _, img := range resp.Images {
		images[*img.ImageID] = img
	}

	return images, nil
}
