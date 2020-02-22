package dal

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/DATA-DOG/go-sqlmock"
)

var DB *sql.DB
var Mock sqlmock.Sqlmock

func TestMain(m *testing.M) {
	var err error

	DB, Mock, err = sqlmock.New()
	if err != nil {
		log.Fatalf("failed while setting up mock db: %s", err.Error())
	}

	code := m.Run()
	DB.Close()

	os.Exit(code)
}

func TestInsertSecretInExistingClient_ShouldInsertSecret(t *testing.T) {
	// Prepare params and expected results
	secret := []byte("my_secret")
	clientID := 1

	// Mock expected SQL queries
	Mock.ExpectExec(`^UPDATE marathon.public.client`).
		WithArgs(secret, clientID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Call the func that we are testing
	rowsAffected, err := InsertSecretInExistingClient(DB, clientID, secret)

	// Assertions
	if err != nil {
		t.Errorf("error was not expected when inserting secret: %s", err)
	}

	// We make sure that all expectations were met
	if err := Mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}

	assert.Equal(t, int64(1), rowsAffected)
}

func TestGetClientSecret_ShouldGetSecret(t *testing.T) {
	// Prepare params and expected results
	clientID := 1
	secret := []byte("my_secret")

	// Mock SQL rows
	cols := []string{
		"secret",
	}
	rows := sqlmock.NewRows(cols).AddRow(secret)

	// Mock expected SQL queries
	expectedSQL := fmt.Sprintf("^SELECT secret FROM client WHERE id = %d*", clientID)
	Mock.ExpectQuery(expectedSQL).WillReturnRows(rows)

	// Call the func that we are testing
	actualSecret, err := GetClientSecret(DB, clientID)

	// Assertions
	if err != nil {
		t.Errorf("error was not expected when getting secret: %s", err)
	}

	// We make sure that all expectations were met
	if err := Mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}

	assert.Equal(t, secret, actualSecret)
}

func TestGetUserTokens_ShouldGetTokens(t *testing.T) {
	platformName := "fitbit"
	userID := 1

	cols := []string{
		"connection_string",
	}
	rows := sqlmock.NewRows(cols).AddRow("oauth2;AC3$$T0K3N;R3FR3$HT0K3N")

	expectedSQL := fmt.Sprintf("^SELECT connection_string FROM credentials WHERE user_id = %d AND platform_name = %q*", userID, platformName)
	Mock.ExpectQuery(expectedSQL).WillReturnRows(rows)

	accessTkn, refreshTkn, err := GetUserTokens(DB, userID, platformName)
	if err != nil {
		t.Errorf("failed to get user tokens: %s", err.Error())
		return
	}

	assert.Equal(t, "AC3$$T0K3N", accessTkn)
	assert.Equal(t, "R3FR3$HT0K3N", refreshTkn)
}

func TestGetPlatformNames(t *testing.T) {
	userID := 1
	expectedPlatforms := []string{"fitbit", "garmin", "google-fit", "map-my-tracks"}

	cols := []string{
		"platform_name",
	}

	rows := sqlmock.NewRows(cols)
	for _, platName := range expectedPlatforms {
		rows = rows.AddRow(platName)
	}

	expectedSQL := fmt.Sprintf(`^SELECT platform_name FROM "credentials" WHERE user_id = %d*`, userID)
	Mock.ExpectQuery(expectedSQL).WillReturnRows(rows)

	platformStr, err := GetPlatformNames(DB, userID)
	if err != nil {
		t.Errorf("failed to get platforms: %s", err.Error())
		return
	}

	assert.Equal(t, expectedPlatforms, platformStr)
}

func TestGetUserByPlatformID(t *testing.T) {
	platID := "A1B2C3"
	platName := "fitbit"
	expectedUserID := 420

	cols := []string{
		"user_id",
	}

	rows := sqlmock.NewRows(cols).AddRow(expectedUserID)

	expectedSQL := fmt.Sprintf(
		"^SELECT user_id FROM credentials [a-z] "+
			"JOIN platform [a-z]+ ON (.+) "+
			"WHERE [a-z]+.name = %q AND [a-z]+.upid = %q*",
		platName,
		platID,
	)
	Mock.ExpectQuery(expectedSQL).WillReturnRows(rows)

	userID, err := GetUserByPlatformID(DB, platID, platName)
	if err != nil {
		t.Errorf("failed to get user: %s", err.Error())
		return
	}

	assert.Equal(t, expectedUserID, userID)
}

func TestInsertUserCredentials_ShouldInsertCredentials(t *testing.T) {
	// Prepare params and expected results
	userID := 1
	clientID := 1
	platName := "fitbit"
	UPID := "A1B2C3"
	connStr := "oauth2;AC3$$T0K3N;R3FR3$HT0K3N"

	// Mock expected DB calls in order
	Mock.ExpectBegin()
	Mock.ExpectQuery(
		`INSERT INTO marathon.public."user"`).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID))

	expectedCredentialsSQL := fmt.Sprintf(
		"^INSERT INTO credentials (.+) VALUES (%d, %q, %q, %q)*",
		clientID, platName, UPID, connStr,
	)
	Mock.ExpectExec(expectedCredentialsSQL).WillReturnResult(sqlmock.NewResult(1, 1))

	expectedUserbaseSQL := fmt.Sprintf(
		`^INSERT INTO userbase (.+) VALUES \(%d, %d\)*`, // Need to escape the parenthesis or else Regex will think it's a capture group
		userID, clientID,
	)
	Mock.ExpectExec(expectedUserbaseSQL).WillReturnResult(sqlmock.NewResult(1, 1))
	Mock.ExpectCommit()

	// Call the func that we are testing
	actualUserID, err := InsertUserCredentials(DB, CredentialParams{
		ClientID:         clientID,
		PlatformName:     platName,
		UPID:             UPID,
		ConnectionString: connStr,
	})

	// Assertions
	if err != nil {
		t.Errorf("error was not expected when inserting user credentials: %s", err)
	}

	// We make sure that all expectations were met
	if err := Mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}

	assert.Equal(t, userID, actualUserID)
}
