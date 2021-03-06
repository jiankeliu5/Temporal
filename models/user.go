package models

import (
	"errors"

	"github.com/jinzhu/gorm"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

/*
	EMAIL ADDRESS MUST BE PROVIDED
*/
type User struct {
	gorm.Model
	EthAddress        string `gorm:"type:varchar(255);unique"`
	EmailAddress      string `gorm:"type:varchar(255);unique"`
	EnterpriseEnabled bool   `gorm:"type:boolean"`
	AccountEnabled    bool   `gorm:"type:boolean"`
	APIAccess         bool   `gorm:"type:boolean"`
	EmailEnabled      bool   `gorm:"type:boolean"`
	HashedPassword    string `gorm:"type:varchar(255)"`
	// IPFSKeyNames is an array of IPFS keys this user has created
	IPFSKeyNames     pq.StringArray `gorm:"type:text[];column:ipfs_key_names"`
	IPFSKeyIDs       pq.StringArray `gorm:"type:text[];column:ipfs_key_ids"`
	IPFSNetworkNames pq.StringArray `gorm:"type:text[];column:ipfs_network_names"`
}

type UserManager struct {
	DB *gorm.DB
}

func NewUserManager(db *gorm.DB) *UserManager {
	um := UserManager{}
	um.DB = db
	return &um
}

func (um *UserManager) GetPrivateIPFSNetworksForUser(ethAddress string) ([]string, error) {
	u := &User{}
	if check := um.DB.Where("eth_address = ?", ethAddress).First(u); check.Error != nil {
		return nil, check.Error
	}
	return u.IPFSNetworkNames, nil
}

func (um *UserManager) CheckIfUserHasAccessToNetwork(ethAddress, networkName string) (bool, error) {
	u := &User{}
	if check := um.DB.Where("eth_address = ?", ethAddress).First(u); check.Error != nil {
		return false, check.Error
	}
	for _, v := range u.IPFSNetworkNames {
		if v == networkName {
			return true, nil
		}
	}
	return false, nil
}
func (um *UserManager) AddIPFSNetworkForUser(ethAddress, networkName string) error {
	u := &User{}
	if check := um.DB.Where("eth_address = ?", ethAddress).First(u); check.Error != nil {
		return check.Error
	}
	for _, v := range u.IPFSNetworkNames {
		if v == networkName {
			return errors.New("network already configured for user")
		}
	}
	u.IPFSNetworkNames = append(u.IPFSNetworkNames, networkName)
	if check := um.DB.Model(u).Update("ipfs_network_names", u.IPFSNetworkNames); check.Error != nil {
		return check.Error
	}

	return nil
}

func (um *UserManager) AddIPFSKeyForUser(ethAddress, keyName, keyID string) error {
	var user User
	if errCheck := um.DB.Where("eth_address = ?", ethAddress).First(&user); errCheck.Error != nil {
		return errCheck.Error
	}

	if user.CreatedAt == nilTime {
		return errors.New("user account does not exist")
	}

	user.IPFSKeyNames = append(user.IPFSKeyNames, keyName)
	user.IPFSKeyIDs = append(user.IPFSKeyIDs, keyID)
	// The following only updates the specified column for the given model
	if errCheck := um.DB.Model(&user).Updates(map[string]interface{}{
		"ipfs_key_names": user.IPFSKeyNames,
		"ipfs_key_ids":   user.IPFSKeyIDs,
	}); errCheck.Error != nil {
		return errCheck.Error
	}
	return nil
}

func (um *UserManager) GetKeysForUser(ethAddress string) (map[string][]string, error) {
	var user User
	keys := make(map[string][]string)
	if errCheck := um.DB.Where("eth_address = ?", ethAddress).First(&user); errCheck.Error != nil {
		return nil, errCheck.Error
	}

	if user.CreatedAt == nilTime {
		return nil, errors.New("user account does not exist")
	}

	keys["key_names"] = user.IPFSKeyNames
	keys["key_ids"] = user.IPFSKeyIDs
	return keys, nil
}

func (um *UserManager) GetKeyIDByName(ethAddress, keyName string) (string, error) {
	var user User
	if errCheck := um.DB.Where("eth_address = ?", ethAddress).First(&user); errCheck.Error != nil {
		return "", errCheck.Error
	}

	if user.CreatedAt == nilTime {
		return "", errors.New("user account does not exist")
	}
	for k, v := range user.IPFSKeyNames {
		if v == keyName {
			return user.IPFSKeyIDs[k], nil
		}
	}
	return "", errors.New("key not found")
}

func (um *UserManager) CheckIfKeyOwnedByUser(ethAddress, keyName string) (bool, error) {
	var user User
	if errCheck := um.DB.Where("eth_address = ?", ethAddress).First(&user); errCheck.Error != nil {
		return false, errCheck.Error
	}

	if user.CreatedAt == nilTime {
		return false, errors.New("user account does not exist")
	}

	for _, v := range user.IPFSKeyNames {
		if v == keyName {
			return true, nil
		}
	}
	return false, nil
}

func (um *UserManager) CheckIfUserAccountEnabled(ethAddress string, db *gorm.DB) (bool, error) {
	var user User
	db.Where("eth_address = ?", ethAddress).First(&user)
	if user.CreatedAt == nilTime {
		return false, errors.New("user account does not exist")
	}
	return user.AccountEnabled, nil
}

// ChangePassword is used to change a users password
func (um *UserManager) ChangePassword(ethAddress, currentPassword, newPassword string) (bool, error) {
	var user User
	um.DB.Where("eth_address = ?", ethAddress).First(&user)
	if user.CreatedAt == nilTime {
		return false, errors.New("user account does not exist")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(currentPassword)); err != nil {
		return false, errors.New("invalid current password")
	}
	newHashedPass, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return false, err
	}
	check := um.DB.Model(&user).Update("hashed_password", string(newHashedPass))
	if check.Error != nil {
		return false, err
	}
	return true, nil
}

func (um *UserManager) NewUserAccount(ethAddress, password, email string, enterpriseEnabled bool) (*User, error) {
	var user User
	um.DB.Where("eth_address = ?", ethAddress).First(&user)
	if user.CreatedAt != nilTime {
		return nil, errors.New("user account already created")
	}
	hashedPass, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user.EthAddress = ethAddress
	user.EnterpriseEnabled = enterpriseEnabled
	user.HashedPassword = string(hashedPass)
	user.EmailAddress = email
	if check := um.DB.Create(&user); check.Error != nil {
		return nil, check.Error
	}
	return &user, nil
}

// SignIn is used to authenticate a user, and check if their account is enabled.
// Returns bool on succesful login, or false with an error on failure
func (um *UserManager) SignIn(ethAddress, password string) (bool, error) {
	var user User
	um.DB.Where("eth_address = ?", ethAddress).First(&user)
	if user.CreatedAt == nilTime {
		return false, errors.New("user account does not exist")
	}
	if !user.AccountEnabled {
		return false, errors.New("account is marked is disabled")
	}
	validPassword, err := um.ComparePlaintextPasswordToHash(ethAddress, password)
	if err != nil {
		return false, err
	}
	if !validPassword {
		return false, errors.New("invalid password supplied")
	}
	return true, nil
}

func (um *UserManager) ComparePlaintextPasswordToHash(ethAddress, password string) (bool, error) {
	var user User
	um.DB.Where("eth_address = ?", ethAddress).First(&user)
	if user.CreatedAt == nilTime {
		return false, errors.New("user account does not exist")
	}
	err := bcrypt.CompareHashAndPassword([]byte(user.HashedPassword), []byte(password))
	if err != nil {
		return false, err
	}
	return true, nil

}

func (um *UserManager) FindByAddress(address string) *User {
	u := User{}
	um.DB.Where("eth_address = ?", address).Find(&u)
	if u.CreatedAt == nilTime {
		return nil
	}
	return &u
}

// FindEmailByAddress is used to find an email address by searching for the users eth address
// the returned map contains their eth address as a key, and their email address as a value
func (um *UserManager) FindEmailByAddress(ethAddress string) (map[string]string, error) {
	u := User{}
	check := um.DB.Where("eth_address = ?", ethAddress).First(&u)
	if check.Error != nil {
		return nil, check.Error
	}
	emails := make(map[string]string)
	emails[ethAddress] = u.EmailAddress
	return emails, nil
}
