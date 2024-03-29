package httpclient

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/Nickolasll/goph-keeper/internal/client/domain"
)

// HTTPClient - Имплементация клиента GophKeeper
type HTTPClient struct {
	client *resty.Client
	log    *logrus.Logger
}

// New - Конструктор нового инстанса клиента
func New(
	log *logrus.Logger,
	cert []byte,
	timeout time.Duration,
	baseURL string,
) *HTTPClient {
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(cert)

	tlsConfig := &tls.Config{
		Renegotiation: tls.RenegotiateOnceAsClient,
		RootCAs:       caCertPool,
		MinVersion:    tls.VersionTLS13,
	}
	client := resty.New().
		SetTLSClientConfig(tlsConfig).
		SetTimeout(timeout).
		SetBaseURL(baseURL)

	return &HTTPClient{
		client: client,
		log:    log,
	}
}

// Login - Вход по логину и паролю, возвращает токен авторизации
func (c HTTPClient) Login(login, password string) (string, error) {
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]any{
			"login":    login,
			"password": password,
		}).Post("/auth/login")

	if err != nil {
		return "", err
	}
	statusCode := resp.StatusCode()
	switch statusCode {
	case http.StatusUnauthorized:
		return "", domain.ErrUnauthorized
	case http.StatusOK:
		return resp.Header().Get("Authorization"), nil
	default:
		c.log.Error(resp.RawResponse)

		return "", domain.ErrClientConnectionError
	}
}

// Register - Регистрация по логину и паролю, возвращает токен авторизации
func (c HTTPClient) Register(login, password string) (string, error) {
	resp, err := c.client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(map[string]any{
			"login":    login,
			"password": password,
		}).Post("/auth/register")

	if err != nil {
		return "", err
	}

	statusCode := resp.StatusCode()
	switch statusCode {
	case http.StatusConflict:
		return "", domain.ErrLoginConflict
	case http.StatusOK:
		return resp.Header().Get("Authorization"), nil
	default:
		c.log.Error(resp.RawResponse)

		return "", domain.ErrClientConnectionError
	}
}

func (c HTTPClient) create(
	authToken, uri, contentType string,
	body any,
) (string, error) {
	resp, err := c.client.R().
		SetHeader("Content-Type", contentType).
		SetHeader("Authorization", authToken).
		SetBody(body).
		Post(uri)

	if err != nil {
		return "", err
	}

	if resp.StatusCode() == http.StatusCreated {
		return resp.Header().Get("Location"), nil
	}

	return "", domain.ErrClientConnectionError
}

func (c HTTPClient) update(
	authToken, uri, contentType string,
	body any,
) error {
	resp, err := c.client.R().
		SetHeader("Content-Type", contentType).
		SetHeader("Authorization", authToken).
		SetBody(body).
		Post(uri)

	if err != nil {
		return err
	}

	statusCode := resp.StatusCode()
	switch statusCode {
	case http.StatusNotFound:
		return domain.ErrEntityNotFound
	case http.StatusBadRequest:
		return domain.ErrBadRequest
	case http.StatusOK:
		return nil
	default:
		c.log.Error(resp.RawResponse)

		return domain.ErrClientConnectionError
	}
}

// CreateText - Создает текст, возвращает идентификатор ресурса от сервера
func (c HTTPClient) CreateText(
	session domain.Session,
	content string,
) (uuid.UUID, error) {
	var uid uuid.UUID
	id, err := c.create(session.Token, "text/create", "plain/text", content)

	if err != nil {
		return uid, err
	}

	uid, err = c.parseID(id)
	if err != nil {
		return uid, err
	}

	return uid, nil
}

// UpdateText - Обновляет существующий текст
func (c HTTPClient) UpdateText(
	session domain.Session,
	text domain.Text,
) error {
	err := c.update(session.Token, "text/"+text.ID.String(), "plain/text", text.Content)

	if err != nil {
		return err
	}

	return nil
}

// GetCerts - Возвращает публичный ключ для валидации и парсинга JWT
func (c HTTPClient) GetCerts() ([]byte, error) {
	resp, err := c.client.R().Get("/auth/certs")

	if err != nil {
		return []byte{}, err
	}

	if resp.StatusCode() == http.StatusOK {
		return resp.Body(), nil
	}

	c.log.Error(resp.RawResponse)

	return []byte{}, domain.ErrClientConnectionError
}

// CreateBinary - Создает бинарные данные, возвращает идентификатор ресурса от сервера
func (c HTTPClient) CreateBinary(
	session domain.Session,
	content []byte,
) (uuid.UUID, error) {
	var uid uuid.UUID
	id, err := c.create(session.Token, "binary/create", "multipart/form-data", content)

	if err != nil {
		return uid, err
	}

	uid, err = c.parseID(id)
	if err != nil {
		return uid, err
	}

	return uid, nil
}

// UpdateBinary - Обновляет существующие бинарные данные
func (c HTTPClient) UpdateBinary(
	session domain.Session,
	bin domain.Binary,
) error {
	err := c.update(session.Token, "binary/"+bin.ID.String(), "multipart/form-data", bin.Content)

	if err != nil {
		return err
	}

	return nil
}

func (c HTTPClient) parseID(id string) (uuid.UUID, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return uid, err
	}

	return uid, nil
}

// CreateCredentials - Создает пару логин и парль, возвращает идентификатор ресурса от сервера
func (c HTTPClient) CreateCredentials(
	session domain.Session,
	name, login, password, meta string,
) (uuid.UUID, error) {
	var uid uuid.UUID
	payload, err := credentialsToJSON(name, login, password, meta)
	if err != nil {
		return uid, err
	}
	id, err := c.create(session.Token, "credentials/create", "application/json", payload)

	if err != nil {
		return uid, err
	}

	uid, err = c.parseID(id)
	if err != nil {
		return uid, err
	}

	return uid, nil
}

// UpdateCredentials - Обновляет существующие логин и пароль
func (c HTTPClient) UpdateCredentials(
	session domain.Session,
	cred *domain.Credentials,
) error {
	payload, err := credentialsToJSON(cred.Name, cred.Login, cred.Password, cred.Meta)
	if err != nil {
		return err
	}
	err = c.update(session.Token, "credentials/"+cred.ID.String(), "application/json", payload)

	if err != nil {
		return err
	}

	return nil
}

// CreateBankCard - Создает банковскую карту, возвращает идентификатор ресурса от сервера
func (c HTTPClient) CreateBankCard(
	session domain.Session,
	number, validThru, cvv, cardHolder, meta string,
) (uuid.UUID, error) {
	var uid uuid.UUID
	payload, err := bankCardToJSON(number, validThru, cvv, cardHolder, meta)
	if err != nil {
		return uid, err
	}
	id, err := c.create(session.Token, "bank_card/create", "application/json", payload)

	if err != nil {
		return uid, err
	}

	uid, err = c.parseID(id)
	if err != nil {
		return uid, err
	}

	return uid, nil
}

// UpdateBankCard - Обновляет существующую банковскую карту
func (c HTTPClient) UpdateBankCard(
	session domain.Session,
	card *domain.BankCard,
) error {
	payload, err := bankCardToJSON(
		card.Number,
		card.ValidThru,
		card.CVV,
		card.CardHolder,
		card.Meta,
	)
	if err != nil {
		return err
	}
	err = c.update(session.Token, "bank_card/"+card.ID.String(), "application/json", payload)

	if err != nil {
		return err
	}

	return nil
}

func (c HTTPClient) parseErrorResponse(body []byte) error {
	errorResp := errorResponse{}
	err := json.Unmarshal(body, &errorResp)
	if err != nil {
		return err
	}

	return errors.New(errorResp.Message)
}

// GetAllTexts - Получает все расшифрованные тексты пользователя
func (c HTTPClient) GetAllTexts(session domain.Session) ([]domain.Text, error) {
	resp, err := c.client.R().
		SetHeader("Authorization", session.Token).
		Get("text/all")

	if err != nil {
		return []domain.Text{}, err
	}

	statusCode := resp.StatusCode()

	if statusCode == http.StatusInternalServerError {
		err := c.parseErrorResponse(resp.Body())

		return []domain.Text{}, err
	}

	if statusCode == http.StatusOK {
		respData := getAllTextsResponse{}
		err = json.Unmarshal(resp.Body(), &respData)
		if err != nil {
			return []domain.Text{}, err
		}

		return respData.Data.Texts, nil
	}

	c.log.Error(resp.RawResponse)

	return []domain.Text{}, domain.ErrClientConnectionError
}

// GetAllBinaries - Получает все расшифрованные бинарные данные пользователя
func (c HTTPClient) GetAllBinaries(session domain.Session) ([]domain.Binary, error) {
	resp, err := c.client.R().
		SetHeader("Authorization", session.Token).
		Get("binary/all")

	if err != nil {
		return []domain.Binary{}, err
	}

	statusCode := resp.StatusCode()

	if statusCode == http.StatusInternalServerError {
		err := c.parseErrorResponse(resp.Body())

		return []domain.Binary{}, err
	}

	if statusCode == http.StatusOK {
		respData := getAllBinariesResponse{}
		err = json.Unmarshal(resp.Body(), &respData)
		if err != nil {
			return []domain.Binary{}, err
		}

		return respData.Data.Binaries, nil
	}

	c.log.Error(resp.RawResponse)

	return []domain.Binary{}, domain.ErrClientConnectionError
}

// GetAllCredentials - Получает все расшифрованные логины и пароли пользователя
func (c HTTPClient) GetAllCredentials(session domain.Session) (creds []domain.Credentials, err error) {
	resp, err := c.client.R().
		SetHeader("Authorization", session.Token).
		Get("credentials/all")

	if err != nil {
		return creds, err
	}

	statusCode := resp.StatusCode()

	if statusCode == http.StatusInternalServerError {
		err = c.parseErrorResponse(resp.Body())

		return creds, err
	}

	if statusCode == http.StatusOK {
		respData := getAllCredentialsResponse{}
		err = json.Unmarshal(resp.Body(), &respData)
		if err != nil {
			return creds, err
		}

		return respData.Data.Credentials, nil
	}

	c.log.Error(resp.RawResponse)

	return creds, domain.ErrClientConnectionError
}

// GetAllBankCards - Получает все расшифрованные банковские карты пользователя
func (c HTTPClient) GetAllBankCards(session domain.Session) (cards []domain.BankCard, err error) {
	resp, err := c.client.R().
		SetHeader("Authorization", session.Token).
		Get("bank_card/all")

	if err != nil {
		return cards, err
	}

	statusCode := resp.StatusCode()

	if statusCode == http.StatusInternalServerError {
		err = c.parseErrorResponse(resp.Body())

		return cards, err
	}

	if statusCode == http.StatusOK {
		respData := getAllBankCardsResponse{}
		err = json.Unmarshal(resp.Body(), &respData)
		if err != nil {
			return cards, err
		}

		return respData.Data.BankCards, nil
	}

	c.log.Error(resp.RawResponse)

	return cards, domain.ErrClientConnectionError
}

// GetAll - Получает все расшифрованные данные пользователя
func (c HTTPClient) GetAll(session domain.Session) (
	texts []domain.Text,
	bankCards []domain.BankCard,
	binaries []domain.Binary,
	credentials []domain.Credentials,
	err error,
) {
	resp, err := c.client.R().
		SetHeader("Authorization", session.Token).
		Get("all")

	if err != nil {
		return texts, bankCards, binaries, credentials, err
	}

	statusCode := resp.StatusCode()

	if statusCode == http.StatusInternalServerError {
		err = c.parseErrorResponse(resp.Body())

		return texts, bankCards, binaries, credentials, err
	}

	if statusCode == http.StatusOK {
		respData := getAllResponse{}
		err = json.Unmarshal(resp.Body(), &respData)
		if err != nil {
			return texts, bankCards, binaries, credentials, err
		}

		data := respData.Data

		return data.Texts, data.BankCards, data.Binaries, data.Credentials, nil
	}

	c.log.Error(resp.RawResponse)

	return texts, bankCards, binaries, credentials, domain.ErrClientConnectionError
}
