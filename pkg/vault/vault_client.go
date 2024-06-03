

package vault

type Client interface {
    ReadSecret(path string) (map[string]interface{}, error)
    WriteSecret(path string, data map[string]interface{}) error
}
