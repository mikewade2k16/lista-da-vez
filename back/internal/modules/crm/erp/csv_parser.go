package erp

import (
	"bufio"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

const csvFilenameTimestampLayout = "20060102150405"

type csvHeaderMode int

const (
	csvHeaderOptional csvHeaderMode = iota
	csvHeaderRequired
	csvHeaderForbidden
)

type csvSourceEncoding string

const (
	csvSourceEncodingUTF8   csvSourceEncoding = "utf-8"
	csvSourceEncodingCP1252 csvSourceEncoding = "cp1252"
)

type csvFileMetadata struct {
	ExtractedAt   time.Time
	StoreCode     string
	StoreCNPJ     string
	DataType      string
	DataReference time.Time
	OriginalName  string
}

var csvFilenamePattern = regexp.MustCompile(`^(\d{14})_(\d{3,5})-(\d{14})-(item|customer|employee|order|ordercanceled)-(\d{14})\.csv$`)
var csvFilenamePatternWithoutExtractedAt = regexp.MustCompile(`^(\d{3,5})-(\d{14})-(item|customer|employee|order|ordercanceled)-(\d{14})\.csv$`)

var expectedColumnsByType = map[string][]string{
	DataTypeItem: {
		"sku",
		"name",
		"description",
		"supplierreference",
		"brandname",
		"seasonname",
		"category1",
		"category2",
		"category3",
		"size",
		"color",
		"unit",
		"price",
		"identifier",
		"created_at",
		"updated_at",
	},
	DataTypeCustomer: {
		"name",
		"nickname",
		"cpf",
		"email",
		"phone",
		"mobile",
		"gender",
		"birthday",
		"street",
		"number",
		"complement",
		"neighborhood",
		"city",
		"uf",
		"country",
		"zipcode",
		"employee_id",
		"store_id",
		"registered_at",
		"original_id",
		"identifier",
		"tags",
	},
	DataTypeEmployee: {
		"name",
		"store_id",
		"original_id",
		"street",
		"complement",
		"city",
		"uf",
		"zipcode",
		"is_active",
	},
	DataTypeOrder: {
		"order_id",
		"identifier",
		"store_id",
		"customer_id",
		"order_date",
		"total_amount",
		"product_return",
		"sku",
		"amount",
		"quantity",
		"employee_id",
		"payment_type",
		"total_exclusion",
		"total_debit",
	},
	DataTypeOrderCanceled: {
		"order_id",
		"identifier",
		"store_id",
		"customer_id",
		"order_date",
		"total_amount",
		"product_return",
		"sku",
		"amount",
		"quantity",
		"employee_id",
		"payment_type",
		"total_exclusion",
		"total_debit",
	},
}

var csvHeaderModesByType = map[string]csvHeaderMode{
	DataTypeItem:          csvHeaderOptional,
	DataTypeCustomer:      csvHeaderOptional,
	DataTypeEmployee:      csvHeaderOptional,
	DataTypeOrder:         csvHeaderOptional,
	DataTypeOrderCanceled: csvHeaderOptional,
}

func parseCSVFilename(name string) (csvFileMetadata, error) {
	baseName := filepath.Base(strings.TrimSpace(name))
	match := csvFilenamePattern.FindStringSubmatch(baseName)
	if match != nil {
		return buildCSVFileMetadata(baseName, match[1], match[2], match[3], match[4], match[5])
	}

	match = csvFilenamePatternWithoutExtractedAt.FindStringSubmatch(baseName)
	if match != nil {
		return buildCSVFileMetadata(baseName, "", match[1], match[2], match[3], match[4])
	}

	return csvFileMetadata{}, &ErrCSVFilenameInvalid{Name: baseName}
}

func buildCSVFileMetadata(baseName string, extractedAtRaw string, storeCode string, storeCNPJ string, dataType string, dataReferenceRaw string) (csvFileMetadata, error) {
	dataReference, err := time.ParseInLocation(csvFilenameTimestampLayout, dataReferenceRaw, time.UTC)
	if err != nil {
		return csvFileMetadata{}, &ErrCSVFilenameInvalid{Name: baseName, Cause: err}
	}

	var extractedAt time.Time
	if strings.TrimSpace(extractedAtRaw) != "" {
		extractedAt, err = time.ParseInLocation(csvFilenameTimestampLayout, extractedAtRaw, time.UTC)
		if err != nil {
			return csvFileMetadata{}, &ErrCSVFilenameInvalid{Name: baseName, Cause: err}
		}
	}

	return csvFileMetadata{
		ExtractedAt:   extractedAt,
		StoreCode:     storeCode,
		StoreCNPJ:     storeCNPJ,
		DataType:      dataType,
		DataReference: dataReference,
		OriginalName:  baseName,
	}, nil
}

func StreamCSV(reader io.Reader, dataType string, meta csvFileMetadata, onRow func(idx int, rec any) error) (string, int, error) {
	return StreamCSVWithLimit(reader, dataType, meta, 0, onRow)
}

func StreamCSVWithLimit(reader io.Reader, dataType string, meta csvFileMetadata, maxBytes int64, onRow func(idx int, rec any) error) (string, int, error) {
	normalizedDataType := strings.TrimSpace(strings.ToLower(dataType))
	if normalizedDataType == "" {
		normalizedDataType = strings.TrimSpace(strings.ToLower(meta.DataType))
	}
	if !isSupportedDataType(normalizedDataType) {
		return "", 0, ErrUnsupportedDataType
	}
	if strings.TrimSpace(meta.DataType) != "" && strings.TrimSpace(meta.DataType) != normalizedDataType {
		return "", 0, fmt.Errorf("%w: filename data type %s differs from requested %s", ErrValidation, meta.DataType, normalizedDataType)
	}
	meta.DataType = normalizedDataType

	tempFile, err := os.CreateTemp("", "erp-csv-*")
	if err != nil {
		return "", 0, err
	}
	defer func() {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
	}()

	hasher := sha256.New()
	copyReader := reader
	if maxBytes > 0 {
		copyReader = io.LimitReader(reader, maxBytes+1)
	}
	bytesCopied, err := io.Copy(io.MultiWriter(tempFile, hasher), copyReader)
	if err != nil {
		return "", 0, err
	}
	if maxBytes > 0 && bytesCopied > maxBytes {
		return "", 0, &ErrCSVTooLarge{
			SourceName: meta.OriginalName,
			MaxBytes:   maxBytes,
			GotBytes:   bytesCopied,
		}
	}
	checksum := hex.EncodeToString(hasher.Sum(nil))

	encoding, err := detectCSVEncoding(tempFile)
	if err != nil {
		return checksum, 0, &ErrCSVEncoding{SourceName: meta.OriginalName, Cause: err}
	}

	rowCount, err := parseCSVRows(tempFile, encoding, normalizedDataType, meta, onRow)
	if err != nil {
		return checksum, rowCount, err
	}

	return checksum, rowCount, nil
}

func detectCSVEncoding(file *os.File) (csvSourceEncoding, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", err
	}

	reader := bufio.NewReader(file)
	for {
		r, size, err := reader.ReadRune()
		if err == io.EOF {
			return csvSourceEncodingUTF8, nil
		}
		if err != nil {
			return "", err
		}
		if r == utf8.RuneError && size == 1 {
			return csvSourceEncodingCP1252, nil
		}
	}
}

func parseCSVRows(file *os.File, encoding csvSourceEncoding, dataType string, meta csvFileMetadata, onRow func(idx int, rec any) error) (int, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	var decoded io.Reader = file
	if encoding == csvSourceEncodingCP1252 {
		decoded = transform.NewReader(file, charmap.Windows1252.NewDecoder())
	}

	reader := csv.NewReader(decoded)
	reader.Comma = ';'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	expectedColumns := expectedColumnsByType[dataType]
	headerMode := csvHeaderModesByType[dataType]
	headerProcessed := false
	rowCount := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return rowCount, &ErrCSVRowParse{SourceName: meta.OriginalName, DataType: dataType, LineNumber: rowCount + 1, Field: "row", Cause: err}
		}
		if len(record) > 0 {
			record[0] = strings.TrimPrefix(record[0], "\ufeff")
		}
		if isBlankCSVRecord(record) {
			continue
		}

		if !headerProcessed {
			headerProcessed = true
			matchesHeader := equalCSVColumns(record, expectedColumns)
			switch {
			case matchesHeader && headerMode == csvHeaderForbidden:
				return rowCount, &ErrCSVHeaderMismatch{SourceName: meta.OriginalName, DataType: dataType, Expected: expectedColumns, Got: record}
			case matchesHeader:
				continue
			case !matchesHeader && headerMode == csvHeaderRequired:
				return rowCount, &ErrCSVHeaderMismatch{SourceName: meta.OriginalName, DataType: dataType, Expected: expectedColumns, Got: record}
			}
		}

		rowIndex := rowCount + 1
		parsedRecord, err := parseCSVRecord(dataType, record, meta, rowIndex)
		if err != nil {
			return rowCount, err
		}
		if err := onRow(rowIndex, parsedRecord); err != nil {
			return rowCount, err
		}
		rowCount++
	}

	return rowCount, nil
}

func parseCSVRecord(dataType string, values []string, meta csvFileMetadata, lineNumber int) (any, error) {
	switch dataType {
	case DataTypeItem:
		return parseItemCSVRecord(values, meta, lineNumber)
	case DataTypeCustomer:
		return parseCustomerCSVRecord(values, meta, lineNumber)
	case DataTypeEmployee:
		return parseEmployeeCSVRecord(values, meta, lineNumber)
	case DataTypeOrder, DataTypeOrderCanceled:
		return parseOrderCSVRecord(values, meta, lineNumber)
	default:
		return nil, ErrUnsupportedDataType
	}
}

func parseItemCSVRecord(values []string, meta csvFileMetadata, lineNumber int) (ItemRawRecord, error) {
	if err := validateCSVColumnCount(meta, DataTypeItem, lineNumber, values); err != nil {
		return ItemRawRecord{}, err
	}
	rawValues, rawPayload := buildRawCSVMirror(expectedColumnsByType[DataTypeItem], values)

	priceCents, err := parseOptionalInt64(values[12])
	if err != nil {
		return ItemRawRecord{}, wrapCSVFieldError(meta, DataTypeItem, lineNumber, "price", err)
	}
	createdAt, err := parseOptionalTimestamp(values[14])
	if err != nil {
		return ItemRawRecord{}, wrapCSVFieldError(meta, DataTypeItem, lineNumber, "created_at", err)
	}
	updatedAt, err := parseOptionalTimestamp(values[15])
	if err != nil {
		return ItemRawRecord{}, wrapCSVFieldError(meta, DataTypeItem, lineNumber, "updated_at", err)
	}

	return ItemRawRecord{
		StoreCode:         meta.StoreCode,
		StoreCNPJ:         meta.StoreCNPJ,
		SourceFileName:    meta.OriginalName,
		SourceBatchDate:   formatCSVBatchDate(meta),
		SourceLineNumber:  lineNumber,
		RawValues:         rawValues,
		RawPayload:        rawPayload,
		SKU:               strings.TrimSpace(values[0]),
		Name:              strings.TrimSpace(values[1]),
		Description:       strings.TrimSpace(values[2]),
		SupplierReference: strings.TrimSpace(values[3]),
		BrandName:         strings.TrimSpace(values[4]),
		SeasonName:        strings.TrimSpace(values[5]),
		Category1:         strings.TrimSpace(values[6]),
		Category2:         strings.TrimSpace(values[7]),
		Category3:         strings.TrimSpace(values[8]),
		Size:              strings.TrimSpace(values[9]),
		Color:             strings.TrimSpace(values[10]),
		Unit:              strings.TrimSpace(values[11]),
		PriceRaw:          strings.TrimSpace(values[12]),
		PriceCents:        priceCents,
		Identifier:        strings.TrimSpace(values[13]),
		CreatedAtRaw:      strings.TrimSpace(values[14]),
		UpdatedAtRaw:      strings.TrimSpace(values[15]),
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func parseCustomerCSVRecord(values []string, meta csvFileMetadata, lineNumber int) (CustomerRawRecord, error) {
	if err := validateCSVColumnCount(meta, DataTypeCustomer, lineNumber, values); err != nil {
		return CustomerRawRecord{}, err
	}
	rawValues, rawPayload := buildRawCSVMirror(expectedColumnsByType[DataTypeCustomer], values)
	if _, err := parseCompactDate(values[7]); err != nil {
		return CustomerRawRecord{}, wrapCSVFieldError(meta, DataTypeCustomer, lineNumber, "birthday", err)
	}
	if _, err := parseCompactDate(values[18]); err != nil {
		return CustomerRawRecord{}, wrapCSVFieldError(meta, DataTypeCustomer, lineNumber, "registered_at", err)
	}

	return CustomerRawRecord{
		StoreCode:        meta.StoreCode,
		StoreCNPJ:        meta.StoreCNPJ,
		SourceFileName:   meta.OriginalName,
		SourceBatchDate:  formatCSVBatchDate(meta),
		SourceLineNumber: lineNumber,
		RawValues:        rawValues,
		RawPayload:       rawPayload,
		Name:             strings.TrimSpace(values[0]),
		Nickname:         strings.TrimSpace(values[1]),
		CPF:              strings.TrimSpace(values[2]),
		Email:            strings.TrimSpace(values[3]),
		Phone:            strings.TrimSpace(values[4]),
		Mobile:           strings.TrimSpace(values[5]),
		Gender:           strings.TrimSpace(values[6]),
		BirthdayRaw:      strings.TrimSpace(values[7]),
		Street:           strings.TrimSpace(values[8]),
		Number:           strings.TrimSpace(values[9]),
		Complement:       strings.TrimSpace(values[10]),
		Neighborhood:     strings.TrimSpace(values[11]),
		City:             strings.TrimSpace(values[12]),
		UF:               strings.TrimSpace(values[13]),
		Country:          strings.TrimSpace(values[14]),
		Zipcode:          strings.TrimSpace(values[15]),
		EmployeeID:       strings.TrimSpace(values[16]),
		StoreIDRaw:       strings.TrimSpace(values[17]),
		RegisteredAtRaw:  strings.TrimSpace(values[18]),
		OriginalID:       strings.TrimSpace(values[19]),
		Identifier:       strings.TrimSpace(values[20]),
		Tags:             strings.TrimSpace(values[21]),
	}, nil
}

func parseEmployeeCSVRecord(values []string, meta csvFileMetadata, lineNumber int) (EmployeeRawRecord, error) {
	if err := validateCSVColumnCount(meta, DataTypeEmployee, lineNumber, values); err != nil {
		return EmployeeRawRecord{}, err
	}
	rawValues, rawPayload := buildRawCSVMirror(expectedColumnsByType[DataTypeEmployee], values)

	return EmployeeRawRecord{
		StoreCode:        meta.StoreCode,
		StoreCNPJ:        meta.StoreCNPJ,
		SourceFileName:   meta.OriginalName,
		SourceBatchDate:  formatCSVBatchDate(meta),
		SourceLineNumber: lineNumber,
		RawValues:        rawValues,
		RawPayload:       rawPayload,
		Name:             strings.TrimSpace(values[0]),
		StoreIDRaw:       strings.TrimSpace(values[1]),
		OriginalID:       strings.TrimSpace(values[2]),
		Street:           strings.TrimSpace(values[3]),
		Complement:       strings.TrimSpace(values[4]),
		City:             strings.TrimSpace(values[5]),
		UF:               strings.TrimSpace(values[6]),
		Zipcode:          strings.TrimSpace(values[7]),
		IsActiveRaw:      strings.TrimSpace(values[8]),
	}, nil
}

func parseOrderCSVRecord(values []string, meta csvFileMetadata, lineNumber int) (OrderRawRecord, error) {
	if err := validateCSVColumnCount(meta, meta.DataType, lineNumber, values); err != nil {
		return OrderRawRecord{}, err
	}
	rawValues, rawPayload := buildRawCSVMirror(expectedColumnsByType[meta.DataType], values)

	orderDate, err := parseOptionalTimestamp(values[4])
	if err != nil {
		return OrderRawRecord{}, wrapCSVFieldError(meta, meta.DataType, lineNumber, "order_date", err)
	}
	totalAmountCents, err := parseOptionalInt64(values[5])
	if err != nil {
		return OrderRawRecord{}, wrapCSVFieldError(meta, meta.DataType, lineNumber, "total_amount", err)
	}
	productReturnCents, err := parseOptionalInt64(values[6])
	if err != nil {
		return OrderRawRecord{}, wrapCSVFieldError(meta, meta.DataType, lineNumber, "product_return", err)
	}
	amountCents, err := parseOptionalInt64(values[8])
	if err != nil {
		return OrderRawRecord{}, wrapCSVFieldError(meta, meta.DataType, lineNumber, "amount", err)
	}
	quantity, err := parseOptionalInt64(values[9])
	if err != nil {
		return OrderRawRecord{}, wrapCSVFieldError(meta, meta.DataType, lineNumber, "quantity", err)
	}
	totalExclusionCents, err := parseOptionalInt64(values[12])
	if err != nil {
		return OrderRawRecord{}, wrapCSVFieldError(meta, meta.DataType, lineNumber, "total_exclusion", err)
	}
	totalDebitCents, err := parseOptionalInt64(values[13])
	if err != nil {
		return OrderRawRecord{}, wrapCSVFieldError(meta, meta.DataType, lineNumber, "total_debit", err)
	}

	return OrderRawRecord{
		StoreCode:           meta.StoreCode,
		StoreCNPJ:           meta.StoreCNPJ,
		SourceFileName:      meta.OriginalName,
		SourceBatchDate:     formatCSVBatchDate(meta),
		SourceLineNumber:    lineNumber,
		RawValues:           rawValues,
		RawPayload:          rawPayload,
		OrderID:             strings.TrimSpace(values[0]),
		Identifier:          strings.TrimSpace(values[1]),
		StoreIDRaw:          strings.TrimSpace(values[2]),
		CustomerID:          strings.TrimSpace(values[3]),
		OrderDateRaw:        strings.TrimSpace(values[4]),
		OrderDate:           orderDate,
		TotalAmountRaw:      strings.TrimSpace(values[5]),
		TotalAmountCents:    totalAmountCents,
		ProductReturnRaw:    strings.TrimSpace(values[6]),
		ProductReturnCents:  productReturnCents,
		SKU:                 strings.TrimSpace(values[7]),
		AmountRaw:           strings.TrimSpace(values[8]),
		AmountCents:         amountCents,
		QuantityRaw:         strings.TrimSpace(values[9]),
		Quantity:            quantity,
		EmployeeID:          strings.TrimSpace(values[10]),
		PaymentType:         strings.TrimSpace(values[11]),
		TotalExclusionRaw:   strings.TrimSpace(values[12]),
		TotalExclusionCents: totalExclusionCents,
		TotalDebitRaw:       strings.TrimSpace(values[13]),
		TotalDebitCents:     totalDebitCents,
	}, nil
}

func validateCSVColumnCount(meta csvFileMetadata, dataType string, lineNumber int, values []string) error {
	expected := expectedColumnsByType[dataType]
	if len(values) == len(expected) {
		return nil
	}
	return &ErrCSVColumnCountMismatch{
		SourceName: meta.OriginalName,
		DataType:   dataType,
		LineNumber: lineNumber,
		Expected:   len(expected),
		Got:        len(values),
	}
}

func wrapCSVFieldError(meta csvFileMetadata, dataType string, lineNumber int, field string, err error) error {
	return &ErrCSVRowParse{
		SourceName: meta.OriginalName,
		DataType:   dataType,
		LineNumber: lineNumber,
		Field:      field,
		Cause:      err,
	}
}

func equalCSVColumns(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if strings.TrimSpace(strings.ToLower(left[index])) != strings.TrimSpace(strings.ToLower(right[index])) {
			return false
		}
	}
	return true
}

func isBlankCSVRecord(record []string) bool {
	if len(record) == 0 {
		return true
	}
	for _, value := range record {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func formatCSVBatchDate(meta csvFileMetadata) string {
	if meta.DataReference.IsZero() {
		return ""
	}
	return meta.DataReference.UTC().Format("2006-01-02")
}

func buildRawCSVMirror(columns []string, values []string) ([]string, map[string]string) {
	rawValues := append([]string{}, values...)
	rawPayload := make(map[string]string, len(columns))
	for index, column := range columns {
		if index >= len(values) {
			break
		}
		rawPayload[column] = values[index]
	}
	return rawValues, rawPayload
}
