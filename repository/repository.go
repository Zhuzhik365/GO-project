package repository

type TestRepository interface {
	GetHello() string
}

type testRepository struct{}

func NewTestRepository() TestRepository {
	return &testRepository{}
}

func (r *testRepository) GetHello() string {
	return "Hello!"
}
