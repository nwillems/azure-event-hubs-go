package storage

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-event-hubs-go/internal/common"
	"github.com/Azure/azure-event-hubs-go/internal/test"
	"github.com/Azure/azure-sdk-for-go/services/storage/mgmt/2017-10-01/storage"
	"github.com/Azure/azure-storage-blob-go/2016-05-31/azblob"
	"github.com/Azure/go-autorest/autorest/azure"
	azauth "github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/stretchr/testify/suite"
)

type (
	// eventHubSuite encapsulates a end to end test of Event Hubs with build up and tear down of all EH resources
	testSuite struct {
		test.BaseSuite
		accountName  string
		containerURL *azblob.ContainerURL
	}
)

func TestStorage(t *testing.T) {
	suite.Run(t, new(testSuite))
}

func (ts *testSuite) SetupSuite() {
	ts.BaseSuite.SetupSuite()
	ts.accountName = strings.ToLower("ehtest" + ts.SubscriptionID[len(ts.SubscriptionID)-5:])
	if err := ts.ensureStorageAccount(); err != nil {
		ts.T().Fatal(err)
	}
}

func (ts *testSuite) TestToken() {
	containerName := "foo"
	blobName := "bar"
	message := "Hello World!!"
	tokenProvider, err := NewAADSASCredential(ts.SubscriptionID, test.ResourceGroupName, ts.accountName, containerName, AADSASCredentialWithEnvironmentVars())
	if err != nil {
		ts.T().Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pipeline := azblob.NewPipeline(tokenProvider, azblob.PipelineOptions{})
	fooURL, err := url.Parse("https://" + ts.accountName + ".blob." + ts.Env.StorageEndpointSuffix + "/" + containerName)
	if err != nil {
		ts.T().Error(err)
	}

	containerURL := azblob.NewContainerURL(*fooURL, pipeline)
	defer containerURL.Delete(ctx, azblob.ContainerAccessConditions{})
	_, err = containerURL.Create(ctx, azblob.Metadata{}, azblob.PublicAccessNone)
	if err != nil {
		ts.T().Error(err)
	}

	blobURL := containerURL.NewBlobURL(blobName).ToBlockBlobURL()
	_, err = blobURL.PutBlob(ctx, strings.NewReader(message), azblob.BlobHTTPHeaders{}, azblob.Metadata{}, azblob.BlobAccessConditions{})
	if err != nil {
		ts.T().Error(err)
	}
}

func (ts *testSuite) ensureStorageAccount() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := getStorageAccountMgmtClient(ts.SubscriptionID, ts.Env)
	accounts, err := client.ListByResourceGroup(ctx, test.ResourceGroupName)
	if err != nil && accounts.StatusCode != 404 {
		fmt.Println(accounts)
		return err
	}

	for _, account := range *accounts.Value {
		if ts.accountName == *account.Name {
			// provisioned, so return
			fmt.Println(*account.Name)
			return nil
		}
	}

	res, err := client.Create(ctx, test.ResourceGroupName, ts.accountName, storage.AccountCreateParameters{
		Sku: &storage.Sku{
			Name: storage.StandardLRS,
			Tier: storage.Standard,
		},
		Kind:     storage.BlobStorage,
		Location: common.PtrString(test.Location),
		AccountPropertiesCreateParameters: &storage.AccountPropertiesCreateParameters{
			AccessTier: storage.Hot,
		},
	})

	fmt.Println(res)
	fmt.Println(err)

	return err
}

func getStorageAccountMgmtClient(subscriptionID string, env azure.Environment) *storage.AccountsClient {
	client := storage.NewAccountsClientWithBaseURI(env.ResourceManagerEndpoint, subscriptionID)
	a, err := azauth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Fatal(err)
	}
	client.Authorizer = a
	return &client
}
