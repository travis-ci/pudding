package common

import (
	"sort"

	"github.com/mitchellh/goamz/ec2"
)

func ResolveAMI(conn *ec2.EC2, ID string) (*ec2.Image, error) {
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

	return FetchLatestWorkerAMI(conn)
}

func FetchLatestWorkerAMI(conn *ec2.EC2) (*ec2.Image, error) {
	filter := ec2.NewFilter()
	filter.Add("tag:role", "worker")
	allImages, err := conn.Images([]string{}, filter)
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

func GetWorkerInstances(conn *ec2.EC2) (map[string]ec2.Instance, error) {
	filter := ec2.NewFilter()
	filter.Add("tag:role", "worker")
	resp, err := conn.Instances([]string{}, filter)

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
