package emv

import (
	"fmt"
	"strings"
)

// FormatEMVField formata um campo EMV TLV (ID + 2-digit length + value)
func FormatEMVField(id, value string) string {
	length := fmt.Sprintf("%02d", len(value))
	return fmt.Sprintf("%s%s%s", id, length, value)
}

// CalculateCRC16CCITT calcula o checksum CRC-16/CCITT-FALSE (Polinômio 0x1021, Inicial 0xFFFF)
func CalculateCRC16CCITT(payload string) string {
	crc := uint16(0xFFFF)
	for i := 0; i < len(payload); i++ {
		crc ^= uint16(payload[i]) << 8
		for j := 0; j < 8; j++ {
			if (crc & 0x8000) != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return fmt.Sprintf("%04X", crc)
}

// GeneratePixPayload gera a string Copia e Cola EMV oficial do Banco Central do Brasil
func GeneratePixPayload(pixKey, txID, merchantName, merchantCity string, amount float64) string {
	if merchantName == "" {
		merchantName = "CatBrasil Mining"
	}
	if merchantCity == "" {
		merchantCity = "SAO PAULO"
	}
	if len(merchantName) > 25 {
		merchantName = merchantName[:25]
	}
	if len(merchantCity) > 15 {
		merchantCity = merchantCity[:15]
	}

	// 00: Payload Format Indicator
	pfi := FormatEMVField("00", "01")

	// 26: Merchant Account Information - PIX
	gui := FormatEMVField("00", "br.gov.bcb.pix")
	key := FormatEMVField("01", strings.TrimSpace(pixKey))
	mai := FormatEMVField("26", gui+key)

	// 52: Merchant Category Code
	mcc := FormatEMVField("52", "0000")

	// 53: Transaction Currency (986 = Real Brasileiro BRL)
	curr := FormatEMVField("53", "986")

	// 54: Transaction Amount
	var amtStr string
	if amount > 0 {
		amtStr = FormatEMVField("54", fmt.Sprintf("%.2f", amount))
	}

	// 58: Country Code
	country := FormatEMVField("58", "BR")

	// 59: Merchant Name
	mName := FormatEMVField("59", merchantName)

	// 60: Merchant City
	mCity := FormatEMVField("60", merchantCity)

	// 62: Additional Data Field Template (TxID)
	refLabel := FormatEMVField("05", txID)
	addFields := FormatEMVField("62", refLabel)

	// Concatena tudo antes do CRC
	rawPayload := pfi + mai + mcc + curr + amtStr + country + mName + mCity + addFields + "6304"

	// 63: CRC16
	crc := CalculateCRC16CCITT(rawPayload)

	return rawPayload + crc
}
