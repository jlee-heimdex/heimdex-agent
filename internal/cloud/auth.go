package cloud

import "log/slog"

type AuthService interface {
	Login(email, password string) error
	Logout() error
	IsAuthenticated() bool
	GetAccessToken() string
}

type StubAuth struct {
	logger *slog.Logger
}

func NewStubAuth(logger *slog.Logger) *StubAuth {
	return &StubAuth{logger: logger}
}

func (s *StubAuth) Login(email, password string) error {
	s.logger.Info("cloud auth stub: login requested", "email", email)
	return nil
}

func (s *StubAuth) Logout() error {
	s.logger.Info("cloud auth stub: logout requested")
	return nil
}

func (s *StubAuth) IsAuthenticated() bool {
	s.logger.Debug("cloud auth stub: checking auth status")
	return false
}

func (s *StubAuth) GetAccessToken() string {
	return ""
}
