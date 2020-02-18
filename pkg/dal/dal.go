package dal

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"errors"

	_ "github.com/lib/pq"
)

type UserCredentials struct {
	PlatformName     string
	PlatformId       string
	ConnectionString string
}

func InitializeDBConn(host string, port int, user, password, dbName string) (*sql.DB, error) {
	connectionString := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbName,
	)

	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, err
	}

	// Test connection
	err = db.Ping()
	if err != nil {
		return nil, err
	}

	return db, nil
}

func InsertSecretInExistingClient(db *sql.DB, clientId int, secret []byte) (int64, error) {
	// TODO: Use ExecContext instead
	result, err := db.Exec(
		`UPDATE marathon.public.client
				SET secret = $1
				WHERE id = $2`,
		secret,
		clientId,
	)

	if err != nil {
		return 0, err
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}

func GetClientSecret(db *sql.DB, fromClientId int) ([]byte, error) {
	// TODO: Use QueryRowContext instead
	var secret []byte
	err := db.QueryRow("SELECT secret FROM client WHERE id = " + strconv.Itoa(fromClientId)).Scan(&secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

func CheckUser(db *sql.DB, platformId string, platformName string) (int, error) {
	// query the database for the user we just authorized
	var userId int

	// check if this user exists in the credentials
	queryString := fmt.Sprintf("SELECT user_id FROM credentials WHERE platform_id='%s' AND platform_name='%s'", platformId, platformName)

	// the first thing we need to do is to create a new user in the user table
	err := db.QueryRow(queryString).Scan(&userId)

	if err != nil {
		if err == sql.ErrNoRows {
			// there were no rows, but otherwise no error occurred.
			// Return a zero
			return 0, nil
		} else {
			return 0, err
		}
	}

	// we actually found a result, so this user already exists
	// return the Marathon ID for them

	return userId, nil

}

func InsertUserCredentials(db *sql.DB, credentials *UserCredentials, clientId int) (int, error) {

	// create a new transaction from the database connection
	tx, err := db.Begin()

	if err != nil {
		// something went wrong. Return a 0 userId, and an error
		return 0, err
	}

	// we need to either commit or rollback the transaction after it is done.
	defer func() {
		if err != nil {
			// something went wrong, rollback the transaction
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	var userId int
	// the first thing we need to do is to create a new user in the user table
	err = tx.QueryRow(`INSERT INTO marathon.public.user DEFAULT VALUES RETURNING id`).Scan(&userId)

	if err != nil {
		return 0, err
	}

	// add the user into the credentials table
	credentialsQuery := fmt.Sprintf("INSERT INTO marathon.public.credentials (user_id, platform_name, platform_id, connection_string) VALUES (%d,'%s','%s','%s')", userId, credentials.PlatformName, credentials.PlatformId, credentials.ConnectionString)
	_, err = tx.Exec(
		credentialsQuery,
	)

	if err != nil {
		return 0, err
	}

	// the final step is to add the user to the appropriate row in the userbase table
	userbaseQuery := fmt.Sprintf("INSERT INTO marathon.public.userbase (user_id, client_id) VALUES (%d,%d)", userId, clientId)
	_, err = tx.Exec(
		userbaseQuery,
	)

	if err != nil {
		return 0, err
	}

	return userId, err // err will be update by the deferred func
}

func CreateUserCredentials(platform string, platformID string, connectionParams []string) (UserCredentials, error) {

	credentials := UserCredentials{
		PlatformName: platform,
		PlatformId:   platformID,
	}

	if len(connectionParams) == 0 {
		return credentials, errors.New("must contain non zero amount of connection parameters")
	}

	var sb strings.Builder
	for _, param := range connectionParams {
		sb.WriteString(param + ";")
	}

	credentials.ConnectionString = sb.String()

	return credentials, nil

}

// TODO: Make it so auth type is not hardcoded in the SQL stmt
func GetUserTokens(db *sql.DB, fromUserId int, platform string) (string, string, error) {
	var accessToken, refreshToken string

	stmt := fmt.Sprintf(
		"SELECT o.access_token, o.refresh_token FROM oauth2 o JOIN %q p on o.id = p.oauth2_id WHERE user_id = %d",
		platform,
		fromUserId,
	)

	// TODO: Use QueryRowContext instead
	err := db.QueryRow(stmt).Scan(&accessToken, &refreshToken)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func GetPlatformsString(db *sql.DB, fromUserId int) (string, error) {
	// Get user's platforms
	var platformsStr string

	stmt := fmt.Sprintf(
		`SELECT platforms FROM "user" WHERE id = %d`,
		fromUserId,
	)

	// TODO: Use QueryRowContext instead
	err := db.QueryRow(stmt).Scan(&platformsStr)
	if err != nil {
		return "", err
	}

	return platformsStr, nil
}
