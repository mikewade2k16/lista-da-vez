package erp

import (
	"bufio"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const itemTimestampLayout = "2006-01-02 15:04:05"

var itemConsolidatedColumns = []string{
	"loja",
	"cnpj_loja",
	"arquivo_origem",
	"data_lote",
	"linha_origem",
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
}

var customerConsolidatedColumns = []string{
	"loja",
	"cnpj_loja",
	"arquivo_origem",
	"data_lote",
	"linha_origem",
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
}

var employeeConsolidatedColumns = []string{
	"loja",
	"cnpj_loja",
	"arquivo_origem",
	"data_lote",
	"linha_origem",
	"name",
	"store_id",
	"original_id",
	"street",
	"complement",
	"city",
	"uf",
	"zipcode",
	"is_active",
}

var orderConsolidatedColumns = []string{
	"loja",
	"cnpj_loja",
	"arquivo_origem",
	"data_lote",
	"linha_origem",
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
}

type itemBatchBuilder struct {
	batch   itemConsolidatedBatch
	hasher  hash.Hash
	inData  bool
	hasRows bool
}

func StreamItemConsolidated(path string, onBatch func(itemConsolidatedBatch) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	var (
		builder         *itemBatchBuilder
		sawColumnHeader bool
	)

	flush := func() error {
		if builder == nil {
			return nil
		}
		if len(builder.batch.Rows) == 0 {
			builder = nil
			return nil
		}
		if strings.TrimSpace(builder.batch.SourceFileName) == "" || strings.TrimSpace(builder.batch.BatchDate) == "" {
			return fmt.Errorf("%w: lote sem metadados obrigatorios", ErrValidation)
		}
		builder.batch.ChecksumSHA256 = hex.EncodeToString(builder.hasher.Sum(nil))
		batch := builder.batch
		builder = nil
		return onBatch(batch)
	}

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)

		switch {
		case line == strings.Join(itemConsolidatedColumns, ";"):
			sawColumnHeader = true
		case strings.HasPrefix(line, "## Lote "):
			if err := flush(); err != nil {
				return err
			}
			builder = &itemBatchBuilder{
				batch:  itemConsolidatedBatch{Rows: make([]ItemRawRecord, 0, 512)},
				hasher: sha256.New(),
			}
		case builder != nil && strings.HasPrefix(line, "- `"):
			key, value, ok := parseMetadataLine(line)
			if !ok {
				continue
			}
			switch key {
			case "processado_em":
				builder.batch.ProcessedAt = value
			case "loja":
				if builder.batch.StoreCode == "" {
					builder.batch.StoreCode = value
				}
			case "cnpj_loja":
				builder.batch.StoreCNPJ = value
			case "arquivo_origem":
				builder.batch.SourceFileName = value
			case "data_lote":
				builder.batch.BatchDate = value
			}
		case builder != nil && line == "### DadosCSV":
			builder.inData = true
		case builder != nil && builder.inData && line == "" && builder.hasRows:
			builder.inData = false
		case builder != nil && builder.inData && line == "" && !builder.hasRows:
			continue
		case builder != nil && builder.inData:
			row, err := parseItemConsolidatedRow(rawLine)
			if err != nil {
				return err
			}
			if builder.batch.StoreCode == "" {
				builder.batch.StoreCode = row.StoreCode
			}
			if builder.batch.StoreCNPJ == "" {
				builder.batch.StoreCNPJ = row.StoreCNPJ
			}
			if builder.batch.SourceFileName == "" {
				builder.batch.SourceFileName = row.SourceFileName
			}
			if builder.batch.BatchDate == "" {
				builder.batch.BatchDate = row.SourceBatchDate
			}
			builder.batch.Rows = append(builder.batch.Rows, row)
			builder.hasRows = true
			_, _ = io.WriteString(builder.hasher, rawLine)
			_, _ = io.WriteString(builder.hasher, "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	if !sawColumnHeader {
		return fmt.Errorf("%w: cabecalho do consolidado de item nao encontrado", ErrValidation)
	}
	return flush()
}

func StreamCustomerConsolidated(path string, onBatch func(customerConsolidatedBatch) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	var (
		batch           customerConsolidatedBatch
		hasher          hash.Hash
		inData          bool
		hasRows         bool
		hasBatch        bool
		sawColumnHeader bool
	)

	flush := func() error {
		if !hasBatch || len(batch.Rows) == 0 {
			return nil
		}
		if strings.TrimSpace(batch.SourceFileName) == "" || strings.TrimSpace(batch.BatchDate) == "" {
			return fmt.Errorf("%w: lote customer sem metadados obrigatorios", ErrValidation)
		}
		batch.ChecksumSHA256 = hex.EncodeToString(hasher.Sum(nil))
		batch.DataType = DataTypeCustomer
		out := batch
		batch = customerConsolidatedBatch{}
		hasBatch = false
		return onBatch(out)
	}

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)

		switch {
		case line == strings.Join(customerConsolidatedColumns, ";"):
			sawColumnHeader = true
		case strings.HasPrefix(line, "## Lote "):
			if err := flush(); err != nil {
				return err
			}
			batch = customerConsolidatedBatch{Rows: make([]CustomerRawRecord, 0, 512)}
			hasher = sha256.New()
			inData = false
			hasRows = false
			hasBatch = true
		case hasBatch && strings.HasPrefix(line, "- `"):
			key, value, ok := parseMetadataLine(line)
			if !ok {
				continue
			}
			switch key {
			case "processado_em":
				batch.ProcessedAt = value
			case "loja":
				if batch.StoreCode == "" {
					batch.StoreCode = value
				}
			case "cnpj_loja":
				batch.StoreCNPJ = value
			case "arquivo_origem":
				batch.SourceFileName = value
			case "data_lote":
				batch.BatchDate = value
			}
		case hasBatch && line == "### DadosCSV":
			inData = true
		case hasBatch && inData && line == "" && hasRows:
			inData = false
		case hasBatch && inData && line == "":
			continue
		case hasBatch && inData:
			row, err := parseCustomerConsolidatedRow(rawLine)
			if err != nil {
				return err
			}
			if batch.StoreCode == "" {
				batch.StoreCode = row.StoreCode
			}
			if batch.StoreCNPJ == "" {
				batch.StoreCNPJ = row.StoreCNPJ
			}
			if batch.SourceFileName == "" {
				batch.SourceFileName = row.SourceFileName
			}
			if batch.BatchDate == "" {
				batch.BatchDate = row.SourceBatchDate
			}
			batch.Rows = append(batch.Rows, row)
			hasRows = true
			_, _ = io.WriteString(hasher, rawLine)
			_, _ = io.WriteString(hasher, "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	if !sawColumnHeader {
		return fmt.Errorf("%w: cabecalho do consolidado de customer nao encontrado", ErrValidation)
	}
	return flush()
}

func StreamEmployeeConsolidated(path string, onBatch func(employeeConsolidatedBatch) error) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	var (
		batch           employeeConsolidatedBatch
		hasher          hash.Hash
		inData          bool
		hasRows         bool
		hasBatch        bool
		sawColumnHeader bool
	)

	flush := func() error {
		if !hasBatch || len(batch.Rows) == 0 {
			return nil
		}
		if strings.TrimSpace(batch.SourceFileName) == "" || strings.TrimSpace(batch.BatchDate) == "" {
			return fmt.Errorf("%w: lote employee sem metadados obrigatorios", ErrValidation)
		}
		batch.ChecksumSHA256 = hex.EncodeToString(hasher.Sum(nil))
		batch.DataType = DataTypeEmployee
		out := batch
		batch = employeeConsolidatedBatch{}
		hasBatch = false
		return onBatch(out)
	}

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)

		switch {
		case line == strings.Join(employeeConsolidatedColumns, ";"):
			sawColumnHeader = true
		case strings.HasPrefix(line, "## Lote "):
			if err := flush(); err != nil {
				return err
			}
			batch = employeeConsolidatedBatch{Rows: make([]EmployeeRawRecord, 0, 128)}
			hasher = sha256.New()
			inData = false
			hasRows = false
			hasBatch = true
		case hasBatch && strings.HasPrefix(line, "- `"):
			key, value, ok := parseMetadataLine(line)
			if !ok {
				continue
			}
			switch key {
			case "processado_em":
				batch.ProcessedAt = value
			case "loja":
				if batch.StoreCode == "" {
					batch.StoreCode = value
				}
			case "cnpj_loja":
				batch.StoreCNPJ = value
			case "arquivo_origem":
				batch.SourceFileName = value
			case "data_lote":
				batch.BatchDate = value
			}
		case hasBatch && line == "### DadosCSV":
			inData = true
		case hasBatch && inData && line == "" && hasRows:
			inData = false
		case hasBatch && inData && line == "":
			continue
		case hasBatch && inData:
			row, err := parseEmployeeConsolidatedRow(rawLine)
			if err != nil {
				return err
			}
			if batch.StoreCode == "" {
				batch.StoreCode = row.StoreCode
			}
			if batch.StoreCNPJ == "" {
				batch.StoreCNPJ = row.StoreCNPJ
			}
			if batch.SourceFileName == "" {
				batch.SourceFileName = row.SourceFileName
			}
			if batch.BatchDate == "" {
				batch.BatchDate = row.SourceBatchDate
			}
			batch.Rows = append(batch.Rows, row)
			hasRows = true
			_, _ = io.WriteString(hasher, rawLine)
			_, _ = io.WriteString(hasher, "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	if !sawColumnHeader {
		return fmt.Errorf("%w: cabecalho do consolidado de employee nao encontrado", ErrValidation)
	}
	return flush()
}

func StreamOrderConsolidated(path string, dataType string, onBatch func(orderConsolidatedBatch) error) error {
	if dataType != DataTypeOrder && dataType != DataTypeOrderCanceled {
		return ErrUnsupportedDataType
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	var (
		batch           orderConsolidatedBatch
		hasher          hash.Hash
		inData          bool
		hasRows         bool
		hasBatch        bool
		sawColumnHeader bool
	)

	flush := func() error {
		if !hasBatch || len(batch.Rows) == 0 {
			return nil
		}
		if strings.TrimSpace(batch.SourceFileName) == "" || strings.TrimSpace(batch.BatchDate) == "" {
			return fmt.Errorf("%w: lote order sem metadados obrigatorios", ErrValidation)
		}
		batch.ChecksumSHA256 = hex.EncodeToString(hasher.Sum(nil))
		batch.DataType = dataType
		out := batch
		batch = orderConsolidatedBatch{}
		hasBatch = false
		return onBatch(out)
	}

	for scanner.Scan() {
		rawLine := scanner.Text()
		line := strings.TrimSpace(rawLine)

		switch {
		case line == strings.Join(orderConsolidatedColumns, ";"):
			sawColumnHeader = true
		case strings.HasPrefix(line, "## Lote "):
			if err := flush(); err != nil {
				return err
			}
			batch = orderConsolidatedBatch{Rows: make([]OrderRawRecord, 0, 512)}
			hasher = sha256.New()
			inData = false
			hasRows = false
			hasBatch = true
		case hasBatch && strings.HasPrefix(line, "- `"):
			key, value, ok := parseMetadataLine(line)
			if !ok {
				continue
			}
			switch key {
			case "processado_em":
				batch.ProcessedAt = value
			case "loja":
				if batch.StoreCode == "" {
					batch.StoreCode = value
				}
			case "cnpj_loja":
				batch.StoreCNPJ = value
			case "arquivo_origem":
				batch.SourceFileName = value
			case "data_lote":
				batch.BatchDate = value
			}
		case hasBatch && line == "### DadosCSV":
			inData = true
		case hasBatch && inData && line == "" && hasRows:
			inData = false
		case hasBatch && inData && line == "":
			continue
		case hasBatch && inData:
			row, err := parseOrderConsolidatedRow(rawLine)
			if err != nil {
				return err
			}
			if batch.StoreCode == "" {
				batch.StoreCode = row.StoreCode
			}
			if batch.StoreCNPJ == "" {
				batch.StoreCNPJ = row.StoreCNPJ
			}
			if batch.SourceFileName == "" {
				batch.SourceFileName = row.SourceFileName
			}
			if batch.BatchDate == "" {
				batch.BatchDate = row.SourceBatchDate
			}
			batch.Rows = append(batch.Rows, row)
			hasRows = true
			_, _ = io.WriteString(hasher, rawLine)
			_, _ = io.WriteString(hasher, "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	if !sawColumnHeader {
		return fmt.Errorf("%w: cabecalho do consolidado de order nao encontrado", ErrValidation)
	}
	return flush()
}

func parseMetadataLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "- `") {
		return "", "", false
	}
	trimmed = strings.TrimPrefix(trimmed, "- `")
	separator := strings.Index(trimmed, "`:")
	if separator <= 0 {
		return "", "", false
	}
	key := strings.TrimSpace(trimmed[:separator])
	value := strings.TrimSpace(strings.TrimPrefix(trimmed[separator+2:], ":"))
	return key, value, key != ""
}

func parseItemConsolidatedRow(line string) (ItemRawRecord, error) {
	reader := csv.NewReader(strings.NewReader(line))
	reader.Comma = ';'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	values, err := reader.Read()
	if err != nil {
		return ItemRawRecord{}, err
	}
	if len(values) != len(itemConsolidatedColumns) {
		return ItemRawRecord{}, fmt.Errorf(
			"%w: linha de item com %d colunas; esperado %d",
			ErrValidation,
			len(values),
			len(itemConsolidatedColumns),
		)
	}

	lineNumber, err := strconv.Atoi(strings.TrimSpace(values[4]))
	if err != nil {
		return ItemRawRecord{}, fmt.Errorf("%w: linha_origem invalida", ErrValidation)
	}

	priceCents, err := parseOptionalInt64(values[17])
	if err != nil {
		return ItemRawRecord{}, fmt.Errorf("%w: price invalido", ErrValidation)
	}
	createdAt, err := parseOptionalTimestamp(values[19])
	if err != nil {
		return ItemRawRecord{}, fmt.Errorf("%w: created_at invalido", ErrValidation)
	}
	updatedAt, err := parseOptionalTimestamp(values[20])
	if err != nil {
		return ItemRawRecord{}, fmt.Errorf("%w: updated_at invalido", ErrValidation)
	}

	return ItemRawRecord{
		StoreCode:         strings.TrimSpace(values[0]),
		StoreCNPJ:         strings.TrimSpace(values[1]),
		SourceFileName:    strings.TrimSpace(values[2]),
		SourceBatchDate:   strings.TrimSpace(values[3]),
		SourceLineNumber:  lineNumber,
		SKU:               strings.TrimSpace(values[5]),
		Name:              strings.TrimSpace(values[6]),
		Description:       strings.TrimSpace(values[7]),
		SupplierReference: strings.TrimSpace(values[8]),
		BrandName:         strings.TrimSpace(values[9]),
		SeasonName:        strings.TrimSpace(values[10]),
		Category1:         strings.TrimSpace(values[11]),
		Category2:         strings.TrimSpace(values[12]),
		Category3:         strings.TrimSpace(values[13]),
		Size:              strings.TrimSpace(values[14]),
		Color:             strings.TrimSpace(values[15]),
		Unit:              strings.TrimSpace(values[16]),
		PriceRaw:          strings.TrimSpace(values[17]),
		PriceCents:        priceCents,
		Identifier:        strings.TrimSpace(values[18]),
		CreatedAtRaw:      strings.TrimSpace(values[19]),
		UpdatedAtRaw:      strings.TrimSpace(values[20]),
		CreatedAt:         createdAt,
		UpdatedAt:         updatedAt,
	}, nil
}

func parseCustomerConsolidatedRow(line string) (CustomerRawRecord, error) {
	values, err := parseDelimitedRow(line)
	if err != nil {
		return CustomerRawRecord{}, err
	}
	if len(values) != len(customerConsolidatedColumns) {
		return CustomerRawRecord{}, fmt.Errorf("%w: linha de customer com %d colunas; esperado %d", ErrValidation, len(values), len(customerConsolidatedColumns))
	}

	lineNumber, err := strconv.Atoi(strings.TrimSpace(values[4]))
	if err != nil {
		return CustomerRawRecord{}, fmt.Errorf("%w: linha_origem invalida", ErrValidation)
	}

	return CustomerRawRecord{
		StoreCode:        strings.TrimSpace(values[0]),
		StoreCNPJ:        strings.TrimSpace(values[1]),
		SourceFileName:   strings.TrimSpace(values[2]),
		SourceBatchDate:  strings.TrimSpace(values[3]),
		SourceLineNumber: lineNumber,
		Name:             strings.TrimSpace(values[5]),
		Nickname:         strings.TrimSpace(values[6]),
		CPF:              strings.TrimSpace(values[7]),
		Email:            strings.TrimSpace(values[8]),
		Phone:            strings.TrimSpace(values[9]),
		Mobile:           strings.TrimSpace(values[10]),
		Gender:           strings.TrimSpace(values[11]),
		BirthdayRaw:      strings.TrimSpace(values[12]),
		Street:           strings.TrimSpace(values[13]),
		Number:           strings.TrimSpace(values[14]),
		Complement:       strings.TrimSpace(values[15]),
		Neighborhood:     strings.TrimSpace(values[16]),
		City:             strings.TrimSpace(values[17]),
		UF:               strings.TrimSpace(values[18]),
		Country:          strings.TrimSpace(values[19]),
		Zipcode:          strings.TrimSpace(values[20]),
		EmployeeID:       strings.TrimSpace(values[21]),
		RegisteredAtRaw:  strings.TrimSpace(values[23]),
		OriginalID:       strings.TrimSpace(values[24]),
		Identifier:       strings.TrimSpace(values[25]),
		Tags:             strings.TrimSpace(values[26]),
	}, nil
}

func parseEmployeeConsolidatedRow(line string) (EmployeeRawRecord, error) {
	values, err := parseDelimitedRow(line)
	if err != nil {
		return EmployeeRawRecord{}, err
	}
	if len(values) != len(employeeConsolidatedColumns) {
		return EmployeeRawRecord{}, fmt.Errorf("%w: linha de employee com %d colunas; esperado %d", ErrValidation, len(values), len(employeeConsolidatedColumns))
	}

	lineNumber, err := strconv.Atoi(strings.TrimSpace(values[4]))
	if err != nil {
		return EmployeeRawRecord{}, fmt.Errorf("%w: linha_origem invalida", ErrValidation)
	}

	return EmployeeRawRecord{
		StoreCode:        strings.TrimSpace(values[0]),
		StoreCNPJ:        strings.TrimSpace(values[1]),
		SourceFileName:   strings.TrimSpace(values[2]),
		SourceBatchDate:  strings.TrimSpace(values[3]),
		SourceLineNumber: lineNumber,
		Name:             strings.TrimSpace(values[5]),
		StoreIDRaw:       strings.TrimSpace(values[6]),
		OriginalID:       strings.TrimSpace(values[7]),
		Street:           strings.TrimSpace(values[8]),
		Complement:       strings.TrimSpace(values[9]),
		City:             strings.TrimSpace(values[10]),
		UF:               strings.TrimSpace(values[11]),
		Zipcode:          strings.TrimSpace(values[12]),
		IsActiveRaw:      strings.TrimSpace(values[13]),
	}, nil
}

func parseOrderConsolidatedRow(line string) (OrderRawRecord, error) {
	values, err := parseDelimitedRow(line)
	if err != nil {
		return OrderRawRecord{}, err
	}
	if len(values) != len(orderConsolidatedColumns) {
		return OrderRawRecord{}, fmt.Errorf("%w: linha de order com %d colunas; esperado %d", ErrValidation, len(values), len(orderConsolidatedColumns))
	}

	lineNumber, err := strconv.Atoi(strings.TrimSpace(values[4]))
	if err != nil {
		return OrderRawRecord{}, fmt.Errorf("%w: linha_origem invalida", ErrValidation)
	}

	orderDate, err := parseOptionalTimestamp(values[9])
	if err != nil {
		return OrderRawRecord{}, fmt.Errorf("%w: order_date invalido", ErrValidation)
	}
	totalAmountCents, err := parseOptionalInt64(values[10])
	if err != nil {
		return OrderRawRecord{}, fmt.Errorf("%w: total_amount invalido", ErrValidation)
	}
	productReturnCents, err := parseOptionalInt64(values[11])
	if err != nil {
		return OrderRawRecord{}, fmt.Errorf("%w: product_return invalido", ErrValidation)
	}
	amountCents, err := parseOptionalInt64(values[13])
	if err != nil {
		return OrderRawRecord{}, fmt.Errorf("%w: amount invalido", ErrValidation)
	}
	quantity, err := parseOptionalInt64(values[14])
	if err != nil {
		return OrderRawRecord{}, fmt.Errorf("%w: quantity invalido", ErrValidation)
	}
	totalExclusionCents, err := parseOptionalInt64(values[17])
	if err != nil {
		return OrderRawRecord{}, fmt.Errorf("%w: total_exclusion invalido", ErrValidation)
	}
	totalDebitCents, err := parseOptionalInt64(values[18])
	if err != nil {
		return OrderRawRecord{}, fmt.Errorf("%w: total_debit invalido", ErrValidation)
	}

	return OrderRawRecord{
		StoreCode:           strings.TrimSpace(values[0]),
		StoreCNPJ:           strings.TrimSpace(values[1]),
		SourceFileName:      strings.TrimSpace(values[2]),
		SourceBatchDate:     strings.TrimSpace(values[3]),
		SourceLineNumber:    lineNumber,
		OrderID:             strings.TrimSpace(values[5]),
		Identifier:          strings.TrimSpace(values[6]),
		StoreIDRaw:          strings.TrimSpace(values[7]),
		CustomerID:          strings.TrimSpace(values[8]),
		OrderDateRaw:        strings.TrimSpace(values[9]),
		OrderDate:           orderDate,
		TotalAmountRaw:      strings.TrimSpace(values[10]),
		TotalAmountCents:    totalAmountCents,
		ProductReturnRaw:    strings.TrimSpace(values[11]),
		ProductReturnCents:  productReturnCents,
		SKU:                 strings.TrimSpace(values[12]),
		AmountRaw:           strings.TrimSpace(values[13]),
		AmountCents:         amountCents,
		QuantityRaw:         strings.TrimSpace(values[14]),
		Quantity:            quantity,
		EmployeeID:          strings.TrimSpace(values[15]),
		PaymentType:         strings.TrimSpace(values[16]),
		TotalExclusionRaw:   strings.TrimSpace(values[17]),
		TotalExclusionCents: totalExclusionCents,
		TotalDebitRaw:       strings.TrimSpace(values[18]),
		TotalDebitCents:     totalDebitCents,
	}, nil
}

func parseDelimitedRow(line string) ([]string, error) {
	reader := csv.NewReader(strings.NewReader(line))
	reader.Comma = ';'
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true
	return reader.Read()
}

func parseOptionalInt64(raw string) (*int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func parseOptionalTimestamp(raw string) (*time.Time, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	parsed, err := time.ParseInLocation(itemTimestampLayout, trimmed, time.UTC)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
