package service

import "go-project/repository"

type TestService interface {
	GetMessage() string
}

type testService struct {
	repo repository.TestRepository
}

func NewTestService(repo repository.TestRepository) TestService {
	return &testService{repo: repo}
}

func (s *testService) GetMessage() string {
	return s.repo.GetHello()
}
