package server

type instanceBuilder struct {
}

func newInstanceBuilder(redisURL string) *instanceBuilder {
	return &instanceBuilder{}
}

func (ib *instanceBuilder) Build(b *instanceBuild) (*instanceBuildDetails, error) {
	return nil, nil
}
