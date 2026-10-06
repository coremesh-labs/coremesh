package sdk

import (
	"encoding/json"
	"fmt"
)

// Decode wandelt einen Payload (Request.Payload, Zeilenwerte, Settings) in
// einen konkreten Go-Typ um, z. B. ein Struct mit json-Tags.
func Decode(payload any, dst any) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: payload: %v", ErrInvalidArgument, err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		return fmt.Errorf("%w: payload: %v", ErrInvalidArgument, err)
	}
	return nil
}
