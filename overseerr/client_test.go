package overseerr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	logger := zerolog.Nop()

	tests := []struct {
		name    string
		baseURL string
		apiKey  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid config",
			baseURL: "http://localhost:5055",
			apiKey:  "test-key",
			wantErr: false,
		},
		{
			name:    "missing URL",
			baseURL: "",
			apiKey:  "test-key",
			wantErr: true,
			errMsg:  "URL is required",
		},
		{
			name:    "missing API key",
			baseURL: "http://localhost:5055",
			apiKey:  "",
			wantErr: true,
			errMsg:  "API key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Skip connection test for error cases
			if tt.wantErr {
				_, err := NewClient(tt.baseURL, tt.apiKey, logger)
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				return
			}

			// For valid config, we need a mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "/api/v1/auth/me", r.URL.Path)
				assert.Equal(t, "test-key", r.Header.Get("X-Api-Key"))
				
				resp := map[string]interface{}{
					"id":          1,
					"displayName": "Test User",
				}
				json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			client, err := NewClient(server.URL, tt.apiKey, logger)
			require.NoError(t, err)
			assert.NotNil(t, client)
			assert.Equal(t, server.URL, client.baseURL)
			assert.Equal(t, tt.apiKey, client.apiKey)
		})
	}
}

func TestClientOptions(t *testing.T) {
	logger := zerolog.Nop()
	
	// Mock server for connection test
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":          1,
			"displayName": "Test User",
		})
	}))
	defer server.Close()

	t.Run("with timeout", func(t *testing.T) {
		client, err := NewClient(server.URL, "test-key", logger, WithTimeout(5*time.Second))
		require.NoError(t, err)
		assert.Equal(t, 5*time.Second, client.httpClient.Timeout)
	})

	t.Run("with page size", func(t *testing.T) {
		client, err := NewClient(server.URL, "test-key", logger, WithPageSize(50))
		require.NoError(t, err)
		assert.Equal(t, 50, client.pageSize)
	})

	t.Run("with custom http client", func(t *testing.T) {
		customClient := &http.Client{Timeout: 10 * time.Second}
		client, err := NewClient(server.URL, "test-key", logger, WithHTTPClient(customClient))
		require.NoError(t, err)
		assert.Equal(t, customClient, client.httpClient)
	})
}

func TestRequestStatus(t *testing.T) {
	tests := []struct {
		status   RequestStatus
		expected string
	}{
		{RequestStatusPending, "PENDING"},
		{RequestStatusApproved, "APPROVED"},
		{RequestStatusDeclined, "DECLINED"},
		{RequestStatusProcessing, "PROCESSING"},
		{RequestStatusPartiallyAvailable, "PARTIALLY_AVAILABLE"},
		{RequestStatusAvailable, "AVAILABLE"},
		{RequestStatusFailed, "FAILED"},
		{RequestStatusUnknown, "UNKNOWN"},
		{RequestStatus(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.status.String())
		})
	}
}

func TestMediaType(t *testing.T) {
	assert.True(t, MediaTypeMovie.IsMovie())
	assert.False(t, MediaTypeTV.IsMovie())
}

func TestUser(t *testing.T) {
	tests := []struct {
		name     string
		user     User
		expected string
	}{
		{
			name: "display name available",
			user: User{
				DisplayName:  "John Doe",
				Username:     "johndoe",
				PlexUsername: "john_plex",
				Email:        "john@example.com",
			},
			expected: "John Doe",
		},
		{
			name: "only username available",
			user: User{
				Username:     "johndoe",
				PlexUsername: "john_plex",
				Email:        "john@example.com",
			},
			expected: "johndoe",
		},
		{
			name: "only plex username available",
			user: User{
				PlexUsername: "john_plex",
				Email:        "john@example.com",
			},
			expected: "john_plex",
		},
		{
			name: "only email available",
			user: User{
				Email: "john@example.com",
			},
			expected: "john@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.user.GetDisplayName())
		})
	}
}

func TestMediaRequest(t *testing.T) {
	t.Run("IsMovieRequest", func(t *testing.T) {
		movieReq := MediaRequest{Type: MediaTypeMovie}
		tvReq := MediaRequest{Type: MediaTypeTV}
		
		assert.True(t, movieReq.IsMovieRequest())
		assert.False(t, tvReq.IsMovieRequest())
	})

	t.Run("GetApprover", func(t *testing.T) {
		approver := &User{DisplayName: "Admin"}
		
		tests := []struct {
			name     string
			req      MediaRequest
			expected *User
		}{
			{
				name: "approved with modifier",
				req: MediaRequest{
					Status:     RequestStatusApproved,
					ModifiedBy: approver,
				},
				expected: approver,
			},
			{
				name: "available with modifier",
				req: MediaRequest{
					Status:     RequestStatusAvailable,
					ModifiedBy: approver,
				},
				expected: approver,
			},
			{
				name: "pending with modifier",
				req: MediaRequest{
					Status:     RequestStatusPending,
					ModifiedBy: approver,
				},
				expected: nil,
			},
			{
				name: "approved without modifier",
				req: MediaRequest{
					Status: RequestStatusApproved,
				},
				expected: nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.expected, tt.req.GetApprover())
			})
		}
	})

	t.Run("ToMovieRequest", func(t *testing.T) {
		now := time.Now()
		req := MediaRequest{
			Status: RequestStatusApproved,
			RequestedBy: User{
				DisplayName: "John Doe",
				Email:       "john@example.com",
			},
			ModifiedBy: &User{
				DisplayName: "Admin",
			},
			CreatedAt:     now,
			IsAutoRequest: true,
		}

		movieReq := req.ToMovieRequest()
		assert.Equal(t, "John Doe", movieReq.RequestedBy)
		assert.Equal(t, "john@example.com", movieReq.RequestedByEmail)
		assert.Equal(t, now, movieReq.RequestDate)
		assert.Equal(t, "APPROVED", movieReq.RequestStatus)
		assert.Equal(t, "Admin", movieReq.ApprovedBy)
		assert.True(t, movieReq.IsAutoRequest)
	})
}

func TestPageInfo(t *testing.T) {
	t.Run("NextPage", func(t *testing.T) {
		pi := PageInfo{Page: 2, Pages: 5}
		next, err := pi.NextPage()
		require.NoError(t, err)
		assert.Equal(t, 3, next)

		pi.Page = 5
		_, err = pi.NextPage()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no more pages")
	})
}

func TestRequestsResponse(t *testing.T) {
	t.Run("HasMorePages", func(t *testing.T) {
		resp := RequestsResponse{
			PageInfo: PageInfo{Page: 2, Pages: 5},
		}
		assert.True(t, resp.HasMorePages())

		resp.PageInfo.Page = 5
		assert.False(t, resp.HasMorePages())
	})
}

func TestAPIError(t *testing.T) {
	t.Run("Error message", func(t *testing.T) {
		err := &APIError{
			StatusCode: 404,
			Message:    "Not Found",
		}
		assert.Equal(t, "overseerr API error: status 404: Not Found", err.Error())
	})

	t.Run("IsNotFound", func(t *testing.T) {
		err := &APIError{StatusCode: 404}
		assert.True(t, err.IsNotFound())

		err.StatusCode = 500
		assert.False(t, err.IsNotFound())
	})

	t.Run("IsUnauthorized", func(t *testing.T) {
		tests := []struct {
			code     int
			expected bool
		}{
			{401, true},
			{403, true},
			{404, false},
			{500, false},
		}

		for _, tt := range tests {
			err := &APIError{StatusCode: tt.code}
			assert.Equal(t, tt.expected, err.IsUnauthorized())
		}
	})
}