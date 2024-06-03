

package vault

import (
    "github.com/stretchr/testify/mock"
)

type MockVaultClient struct {
    mock.Mock
}

func (m *MockVaultClient) ReadSecret(path string) (map[string]interface{}, error) {
    args := m.Called(path)
    return args.Get(0).(map[string]interface{}), args.Error(1)
}

func (m *MockVaultClient) WriteSecret(path string, data map[string]interface{}) error {
    args := m.Called(path, data)
    return args.Error(0)
}
