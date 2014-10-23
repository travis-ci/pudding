package common

import (
	"sort"

	"github.com/mitchellh/goamz/ec2"
)

func ResolveAMI(conn *ec2.EC2, ID string) (*ec2.Image, error) {
	if ID != "" {
		resp, err := conn.Images([]string{}, &ec2.Filter{})
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
	allImages, err := conn.Images([]string{}, &ec2.Filter{})
	if err != nil {
		return nil, err
	}

	imgNames := []string{}
	imgMap := map[string]*ec2.Image{}

	for _, img := range imagesWithTag("role", "worker", allImages.Images) {
		imgNames = append(imgNames, img.Name)
		imgMap[img.Name] = img
	}

	sort.Strings(imgNames)
	return imgMap[imgNames[len(imgNames)-1]], nil
}

func imagesWithTag(key, value string, images []ec2.Image) []*ec2.Image {
	out := []*ec2.Image{}
	for _, img := range images {
		for _, tag := range img.Tags {
			if tag.Key == key && tag.Value == value {
				out = append(out, &img)
			}
		}
	}

	return out
}
