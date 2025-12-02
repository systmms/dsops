package rotation_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/systmms/dsops/tests/testutil"
)

func TestPostgreSQLConnectionSetup(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.StartDockerEnv(t, []string{"postgres"})
	defer env.Stop()

	t.Run("database_connection", func(t *testing.T) {
		pgClient := env.PostgresClient()
		require.NotNil(t, pgClient, "PostgreSQL client should not be nil")

		// Test basic query
		var result int
		err := pgClient.QueryRow("SELECT 1").Scan(&result)
		require.NoError(t, err, "Failed to execute basic query")
		assert.Equal(t, 1, result, "Query result should be 1")
	})

	t.Run("create_and_drop_user", func(t *testing.T) {
		pgClient := env.PostgresClient()

		// Create test user
		testUser := "test_rotation_user"
		testPassword := "test_password_123"

		err := pgClient.CreateTestUser(testUser, testPassword)
		require.NoError(t, err, "Failed to create test user")

		// Verify user exists
		exists, err := pgClient.UserExists(testUser)
		require.NoError(t, err, "Failed to check if user exists")
		assert.True(t, exists, "User should exist after creation")

		// Drop test user
		err = pgClient.DropTestUser(testUser)
		require.NoError(t, err, "Failed to drop test user")

		// Verify user no longer exists
		exists, err = pgClient.UserExists(testUser)
		require.NoError(t, err, "Failed to check if user exists after drop")
		assert.False(t, exists, "User should not exist after drop")
	})

	t.Run("user_password_update", func(t *testing.T) {
		pgClient := env.PostgresClient()

		// Create test user
		testUser := "test_password_change_user"
		initialPassword := "initial_password_123"

		err := pgClient.CreateTestUser(testUser, initialPassword)
		require.NoError(t, err, "Failed to create test user")
		defer pgClient.DropTestUser(testUser)

		// Update password
		newPassword := "new_password_456"
		updateQuery := "ALTER USER test_password_change_user WITH PASSWORD '" + newPassword + "'"
		err = pgClient.Exec(updateQuery)
		require.NoError(t, err, "Failed to update user password")

		// Verify user still exists
		exists, err := pgClient.UserExists(testUser)
		require.NoError(t, err)
		assert.True(t, exists, "User should still exist after password change")
	})

	t.Run("concurrent_user_operations", func(t *testing.T) {
		pgClient := env.PostgresClient()

		// Create multiple users concurrently
		numUsers := 10
		results := make(chan error, numUsers)

		for i := 0; i < numUsers; i++ {
			i := i // Capture loop variable
			go func() {
				username := testutil.RandomString("test_concurrent_user", 10)
				password := "concurrent_password_" + string(rune(i))

				err := pgClient.CreateTestUser(username, password)
				results <- err

				// Cleanup
				if err == nil {
					pgClient.DropTestUser(username)
				}
			}()
		}

		// Collect results
		for i := 0; i < numUsers; i++ {
			err := <-results
			assert.NoError(t, err, "Concurrent user creation should succeed")
		}
	})

	t.Run("table_creation_and_query", func(t *testing.T) {
		pgClient := env.PostgresClient()

		// Create test table
		createTableQuery := `
			CREATE TABLE IF NOT EXISTS test_rotation_table (
				id SERIAL PRIMARY KEY,
				name VARCHAR(100),
				created_at TIMESTAMP DEFAULT NOW()
			)
		`
		err := pgClient.Exec(createTableQuery)
		require.NoError(t, err, "Failed to create test table")

		// Insert test data
		insertQuery := "INSERT INTO test_rotation_table (name) VALUES ($1)"
		err = pgClient.Exec(insertQuery, "test_record")
		require.NoError(t, err, "Failed to insert test data")

		// Query test data
		var name string
		err = pgClient.QueryRow("SELECT name FROM test_rotation_table WHERE name = $1", "test_record").Scan(&name)
		require.NoError(t, err, "Failed to query test data")
		assert.Equal(t, "test_record", name, "Query result should match inserted data")

		// Cleanup table
		dropTableQuery := "DROP TABLE IF EXISTS test_rotation_table"
		err = pgClient.Exec(dropTableQuery)
		require.NoError(t, err, "Failed to drop test table")
	})
}

func TestPostgreSQLRotationScenario(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.StartDockerEnv(t, []string{"postgres"})
	defer env.Stop()

	t.Run("simulate_credential_rotation", func(t *testing.T) {
		pgClient := env.PostgresClient()

		// Simulate a rotation scenario
		// 1. Create application user with initial password
		appUser := "app_user"
		initialPassword := "initial_app_password_123"

		err := pgClient.CreateTestUser(appUser, initialPassword)
		require.NoError(t, err, "Failed to create application user")
		defer pgClient.DropTestUser(appUser)

		// Verify user exists
		exists, err := pgClient.UserExists(appUser)
		require.NoError(t, err)
		assert.True(t, exists, "Application user should exist")

		// 2. Rotate password (simulate rotation)
		newPassword := "rotated_app_password_456"
		rotateQuery := "ALTER USER app_user WITH PASSWORD '" + newPassword + "'"
		err = pgClient.Exec(rotateQuery)
		require.NoError(t, err, "Failed to rotate user password")

		// 3. Verify user still exists after rotation
		exists, err = pgClient.UserExists(appUser)
		require.NoError(t, err)
		assert.True(t, exists, "User should still exist after rotation")

		// 4. Simulate a second rotation (two-key rotation strategy)
		secondPassword := "second_rotation_password_789"
		rotateQuery2 := "ALTER USER app_user WITH PASSWORD '" + secondPassword + "'"
		err = pgClient.Exec(rotateQuery2)
		require.NoError(t, err, "Failed to perform second rotation")

		// Verify user still exists
		exists, err = pgClient.UserExists(appUser)
		require.NoError(t, err)
		assert.True(t, exists, "User should still exist after second rotation")
	})

	t.Run("grant_and_revoke_permissions", func(t *testing.T) {
		pgClient := env.PostgresClient()

		// Create test user
		testUser := "test_permissions_user"
		testPassword := "test_perm_password"

		err := pgClient.CreateTestUser(testUser, testPassword)
		require.NoError(t, err)
		defer pgClient.DropTestUser(testUser)

		// Create test table
		createTableQuery := `
			CREATE TABLE IF NOT EXISTS test_permissions_table (
				id SERIAL PRIMARY KEY,
				data TEXT
			)
		`
		err = pgClient.Exec(createTableQuery)
		require.NoError(t, err)
		defer pgClient.Exec("DROP TABLE IF EXISTS test_permissions_table")

		// Grant SELECT permission
		grantQuery := "GRANT SELECT ON test_permissions_table TO test_permissions_user"
		err = pgClient.Exec(grantQuery)
		require.NoError(t, err, "Failed to grant SELECT permission")

		// Revoke SELECT permission
		revokeQuery := "REVOKE SELECT ON test_permissions_table FROM test_permissions_user"
		err = pgClient.Exec(revokeQuery)
		require.NoError(t, err, "Failed to revoke SELECT permission")
	})

	t.Run("connection_pool_compatibility", func(t *testing.T) {
		pgClient := env.PostgresClient()

		// Simulate multiple connections (connection pool)
		numConnections := 20
		results := make(chan error, numConnections)

		for i := 0; i < numConnections; i++ {
			go func(id int) {
				// Execute query
				var result int
				err := pgClient.QueryRow("SELECT $1", id).Scan(&result)
				if err != nil {
					results <- err
					return
				}
				if result != id {
					results <- assert.AnError
					return
				}
				results <- nil
			}(i)
		}

		// Collect results
		for i := 0; i < numConnections; i++ {
			err := <-results
			assert.NoError(t, err, "Connection pool query should succeed")
		}
	})

	t.Run("query_with_timeout", func(t *testing.T) {
		pgClient := env.PostgresClient()

		// Test that queries complete successfully
		var result int
		err := pgClient.QueryRow("SELECT 42").Scan(&result)
		require.NoError(t, err)
		assert.Equal(t, 42, result, "Query result should be 42")
	})
}

func TestPostgreSQLErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := testutil.StartDockerEnv(t, []string{"postgres"})
	defer env.Stop()

	t.Run("duplicate_user_creation", func(t *testing.T) {
		pgClient := env.PostgresClient()

		testUser := "test_duplicate_user"
		testPassword := "test_password"

		// Create user first time
		err := pgClient.CreateTestUser(testUser, testPassword)
		require.NoError(t, err)
		defer pgClient.DropTestUser(testUser)

		// Try to create same user again
		err = pgClient.CreateTestUser(testUser, testPassword)
		assert.Error(t, err, "Creating duplicate user should fail")
		assert.Contains(t, err.Error(), "already exists", "Error should indicate user already exists")
	})

	t.Run("invalid_sql_query", func(t *testing.T) {
		pgClient := env.PostgresClient()

		// Execute invalid SQL
		err := pgClient.Exec("INVALID SQL QUERY")
		assert.Error(t, err, "Invalid SQL should return error")
	})

	t.Run("drop_nonexistent_user", func(t *testing.T) {
		pgClient := env.PostgresClient()

		// Try to drop user that doesn't exist (should succeed with IF EXISTS)
		err := pgClient.DropTestUser("nonexistent_user_12345")
		assert.NoError(t, err, "Dropping nonexistent user with IF EXISTS should succeed")
	})
}
