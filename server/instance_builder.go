package server

type instanceBuilder struct {
	RedisURL string
}

type instanceBuildDetails struct {
	ID  string
	AMI string
}

func newInstanceBuilder(redisURL string) *instanceBuilder {
	return &instanceBuilder{
		RedisURL: redisURL,
	}
}

func (ib *instanceBuilder) Build(b *instanceBuild) (*instanceBuildDetails, error) {
	d := &instanceBuildDetails{}

	return d, nil
}
