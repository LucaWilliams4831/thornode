package types

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gitlab.com/thorchain/thornode/common"
)

// NewKeygenBlock create a new KeygenBlock
func NewKeygenBlock(height int64) KeygenBlock {
	return KeygenBlock{
		Height: height,
	}
}

// IsEmpty determinate whether KeygenBlock is empty
func (m *KeygenBlock) IsEmpty() bool {
	return len(m.Keygens) == 0 && m.Height <= 0
}

// String implement fmt.Stringer print out a string version of keygen block
func (m *KeygenBlock) String() string {
	var keygens []string
	for _, keygen := range m.Keygens {
		keygens = append(keygens, keygen.String())
	}
	return strings.Join(keygens, "\n")
}

// Contains will go through the keygen items and find out whether the given
// keygen already exist in the block or not
func (m *KeygenBlock) Contains(keygen Keygen) bool {
	for _, item := range m.Keygens {
		if item.ID.Equals(keygen.ID) {
			return true
		}
	}
	return false
}

// getKeygenTypeFromString parse the given string as KeygenType
func getKeygenTypeFromString(t string) KeygenType {
	switch {
	case strings.EqualFold(t, "asgardKeygen"):
		return KeygenType_AsgardKeygen
	case strings.EqualFold(t, "yggdrasilKeygen"):
		return KeygenType_YggdrasilKeygen
	}
	return KeygenType_UnknownKeygen
}

// MarshalJSON marshal keygen type to JSON in string form
func (x KeygenType) MarshalJSON() ([]byte, error) {
	return json.Marshal(x.String())
}

// UnmarshalJSON convert string form back to PoolStatus
func (x *KeygenType) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	*x = getKeygenTypeFromString(s)
	return nil
}

// NewKeygen create a new instance of Keygen
func NewKeygen(height int64, members []string, keygenType KeygenType) (Keygen, error) {
	// sort the members
	sort.SliceStable(members, func(i, j int) bool {
		return members[i] < members[j]
	})
	id, err := getKeygenID(height, members, keygenType)
	if err != nil {
		return Keygen{}, fmt.Errorf("fail to create new keygen: %w", err)
	}
	return Keygen{
		ID:      id,
		Members: members,
		Type:    keygenType,
	}, nil
}

// getKeygenID will create ID based on the pub keys
func getKeygenID(height int64, members []string, keygenType KeygenType) (common.TxID, error) {
	sb := strings.Builder{}
	sb.WriteString(strconv.FormatInt(height, 10))
	sb.WriteString(keygenType.String())
	for _, m := range members {
		sb.WriteString(m)
	}
	h := sha256.New()
	_, err := h.Write([]byte(sb.String()))
	if err != nil {
		return "", fmt.Errorf("fail to write to hash: %w", err)
	}

	return common.TxID(hex.EncodeToString(h.Sum(nil))), nil
}

func (m *Keygen) GetMembers() common.PubKeys {
	pubkeys := make(common.PubKeys, 0)
	for _, pk := range m.Members {
		pk, err := common.NewPubKey(pk)
		if err != nil {
			continue
		}
		pubkeys = append(pubkeys, pk)
	}
	return pubkeys
}

// IsEmpty check whether there are any keys in the keygen
func (m *Keygen) IsEmpty() bool {
	return len(m.Members) == 0 || len(m.ID) == 0
}

// Valid is to check whether the keygen members are valid
func (m *Keygen) Valid() error {
	if m.Type == KeygenType_UnknownKeygen {
		return errors.New("unknown keygen")
	}
	return m.GetMembers().Valid()
}

// String implement of fmt.Stringer
func (m *Keygen) String() string {
	return fmt.Sprintf(`id:%s
	type:%s
	member:%+v
`, m.ID, m.Type, m.Members)
}
