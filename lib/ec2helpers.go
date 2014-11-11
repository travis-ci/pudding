package lib

import (
	"sort"

	"github.com/mitchellh/goamz/ec2"
)

// ResolveAMI attempts to get an ec2.Image by id, falling back to
// fetching the most recently provisioned worker ami via
// FetchLatestWorkerAMI
func ResolveAMI(conn *ec2.EC2, ID string, f *ec2.Filter) (*ec2.Image, error) {
	if ID != "" {
		resp, err := conn.Images([]string{ID}, ec2.NewFilter())
		if err != nil {
			return nil, err
		}
		for _, img := range resp.Images {
			if img.Id == ID {
				return &img, nil
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
	f.Add("tag-key", "active")

	allImages, err := conn.Images([]string{}, f)
	if err != nil {
		return nil, err
	}

	imgNames := []string{}
	imgMap := map[string]ec2.Image{}

	for _, img := range allImages.Images {
		imgNames = append(imgNames, img.Name)
		imgMap[img.Name] = img
	}

	sort.Strings(imgNames)
	img := imgMap[imgNames[len(imgNames)-1]]
	return &img, nil
}

// GetInstancesWithFilter fetches all instances that match the
// given filter
func GetInstancesWithFilter(conn *ec2.EC2, f *ec2.Filter) (map[string]ec2.Instance, error) {
	resp, err := conn.Instances([]string{}, f)

	if err != nil {
		return nil, err
	}

	instances := map[string]ec2.Instance{}

	for _, res := range resp.Reservations {
		for _, inst := range res.Instances {
			instances[inst.InstanceId] = inst
		}
	}

	return instances, nil
}
