package models

import (
	"errors"
	"math/big"

	"github.com/jinzhu/gorm"
)

type PinPayment struct {
	gorm.Model
	Method           uint8  `json:"method"`
	Number           string `json:"number"`
	ChargeAmount     string `json:"charge_amount"`
	EthAddress       string `json:"eth_address"`
	ContentHash      string `json:"content_hash"`
	NetworkName      string `json:"network_name"`
	HoldTimeInMonths int64  `json:"hold_time_in_months"`
}

type PinPaymentManager struct {
	DB *gorm.DB
}

func NewPinPaymentManager(db *gorm.DB) *PinPaymentManager {
	return &PinPaymentManager{DB: db}
}

func (ppm *PinPaymentManager) FindPaymentByNumberAndAddress(number, ethAddress string) (*PinPayment, error) {
	pp := &PinPayment{}
	if check := ppm.DB.Where("eth_address = ? AND number = ?", ethAddress, number).First(pp); check.Error != nil {
		return nil, check.Error
	}
	return pp, nil
}

func (ppm *PinPaymentManager) NewPayment(method uint8, number, chargeAmount *big.Int, uploaderAddress, contentHash string, holdTimeInMonths int64) (*PinPayment, error) {
	_, err := ppm.FindPaymentByNumberAndAddress(number.String(), uploaderAddress)
	if err == nil {
		return nil, errors.New("payment already exists")
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}
	pp := &PinPayment{
		Number:           number.String(),
		Method:           method,
		ChargeAmount:     chargeAmount.String(),
		EthAddress:       uploaderAddress,
		ContentHash:      contentHash,
		HoldTimeInMonths: holdTimeInMonths,
	}
	if check := ppm.DB.Create(pp); check.Error != nil {
		return nil, check.Error
	}
	return pp, nil
}

func (ppm *PinPaymentManager) RetrieveLatestPayment(ethAddress string) (*PinPayment, error) {
	pp := PinPayment{}
	if check := ppm.DB.Table("pin_payments").Order("number desc").Where("eth_address = ?", ethAddress).First(&pp); check.Error != nil {
		return nil, check.Error
	}
	return &pp, nil
}

func (ppm *PinPaymentManager) RetrieveLatestPaymentNumber(ethAddress string) (*big.Int, error) {
	pp := &PinPayment{}
	num := big.NewInt(0)
	check := ppm.DB.Table("pin_payments").Order("number desc").Where("eth_address = ?", ethAddress).First(pp)
	if check.Error != nil && check.Error != gorm.ErrRecordNotFound {
		return nil, check.Error
	}
	if check.Error == gorm.ErrRecordNotFound {
		return num, nil
	}
	var valid bool
	num, valid = num.SetString(pp.Number, 10)
	if !valid {
		return nil, errors.New("failed to convert from string to big int")
	}
	return num, nil
}

type FilePayment struct {
	gorm.Model
	Method           uint8
	Number           string
	ChargeAmount     string
	EthAddress       string
	BucketName       string
	ObjectName       string
	NetworkName      string
	HoldTimeInMonths int64
}

type FilePaymentManager struct {
	DB *gorm.DB
}

func NewFilePaymentManager(db *gorm.DB) *FilePaymentManager {
	return &FilePaymentManager{DB: db}
}

func (fpm *FilePaymentManager) NewPayment(method uint8, number, chargeAmount *big.Int, uploaderAddress, bucketName, objectName, networkName string, holdTimeInMonths int64) (*FilePayment, error) {
	fp := &FilePayment{
		Number:           number.String(),
		Method:           method,
		ChargeAmount:     chargeAmount.String(),
		EthAddress:       uploaderAddress,
		BucketName:       bucketName,
		ObjectName:       objectName,
		NetworkName:      networkName,
		HoldTimeInMonths: holdTimeInMonths,
	}
	if check := fpm.DB.Create(fp); check.Error != nil {
		return nil, check.Error
	}
	return fp, nil
}

func (fpm *FilePaymentManager) RetrieveLatestPaymentNumber(ethAddress string) (*big.Int, error) {
	fp := &FilePayment{}
	num := big.NewInt(0)
	check := fpm.DB.Table("file_payments").Order("number desc").Where("eth_address = ?", ethAddress).First(fp)
	if check.Error != nil && check.Error != gorm.ErrRecordNotFound {
		return nil, check.Error
	}
	if check.Error == gorm.ErrRecordNotFound {
		return num, nil
	}
	var valid bool
	num, valid = num.SetString(fp.Number, 10)
	if !valid {
		return nil, errors.New("failed to convert from string to big int")
	}
	return num, nil
}
