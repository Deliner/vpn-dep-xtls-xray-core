package artifact

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"vpn-script/internal/core/model"
	"vpn-script/internal/platform/safeio"
)

func Download(ctx context.Context, asset model.Asset) ([]byte, error) {
	client := downloadClient()
	defer client.CloseIdleConnections()
	return download(ctx, client, asset)
}

func download(parent context.Context, client *http.Client, asset model.Asset) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	if err := validateAsset(asset); err != nil {
		return nil, err
	}
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		contents, err := fetch(ctx, client, asset)
		if err == nil {
			return contents, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		var failure *downloadFailure
		if !errors.As(err, &failure) || !failure.retry || attempt == 2 {
			return nil, err
		}
		if err := waitRetry(ctx, retryDelay(attempt, failure.after)); err != nil {
			return nil, err
		}
	}
}

func fetch(ctx context.Context, client *http.Client, asset model.Asset) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.URL, nil)
	if err != nil {
		return nil, &downloadFailure{message: "invalid runtime HTTPS request", cause: err}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, networkFailure("runtime HTTPS request failed", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &downloadFailure{message: fmt.Sprintf("runtime download HTTP status %d", response.StatusCode),
			retry: transientStatus(response.StatusCode), after: retryAfter(response.Header.Get("Retry-After"))}
	}
	if response.ContentLength >= 0 && response.ContentLength != asset.Size {
		return nil, fmt.Errorf("runtime download declared size mismatch")
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, asset.Size+1))
	if err != nil {
		return nil, networkFailure("runtime HTTPS body read failed", err)
	}
	if int64(len(contents)) != asset.Size || safeio.Hash(contents) != asset.SHA256 {
		return nil, fmt.Errorf("runtime download checksum/size mismatch")
	}
	return contents, nil
}
