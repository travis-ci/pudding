package lib

// Image is the internal representation of an EC2 instance
type Image struct {
	ImageID     string `json:"image_id" redis:"image_id"`
	Role        string `json:"role" redis:"role"`
	Active      bool   `json:"active" redis:"active"`
	Description string `json:"description" redis:"description"`
	State       string `json:"state" redis:"state"`
}
