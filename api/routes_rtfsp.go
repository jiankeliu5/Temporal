package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/RTradeLtd/Temporal/queue"
	"github.com/RTradeLtd/Temporal/rtfs"

	"github.com/RTradeLtd/Temporal/models"
	"github.com/jinzhu/gorm"

	"github.com/RTradeLtd/Temporal/utils"
	"github.com/gin-gonic/gin"
)

// PinToHostedIPFSNetwork is used to pin content to a private/hosted ipfs network
func PinToHostedIPFSNetwork(c *gin.Context) {
	hash := c.Param("hash")
	ethAddress := GetAuthenticatedUserFromContext(c)
	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
	}
	holdTimeInMonths, exists := c.GetPostForm("hold_time")
	if !exists {
		FailNoExistPostForm(c, "hold_time")
		return
	}
	holdTimeInt, err := strconv.ParseInt(holdTimeInMonths, 10, 64)
	if err != nil {
		FailOnError(c, err)
		return
	}

	ip := queue.IPFSPin{
		CID:              hash,
		NetworkName:      networkName,
		EthAddress:       ethAddress,
		HoldTimeInMonths: holdTimeInt,
	}

	mqConnectionURL, ok := c.MustGet("mq_conn_url").(string)
	if !ok {
		FailOnError(c, errors.New("unable to load rabbitmq"))
		return
	}

	qm, err := queue.Initialize(queue.IpfsPinQueue, mqConnectionURL)
	if err != nil {
		FailOnError(c, err)
		return
	}

	err = qm.PublishMessageWithExchange(ip, queue.PinExchange)
	if err != nil {
		FailOnError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": "content pin request sent to backend",
	})
}

func GetFileSizeInBytesForObjectForHostedIPFSNetwork(c *gin.Context) {
	ethAddress := GetAuthenticatedUserFromContext(c)
	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}
	err := CheckAccessForPrivateNetwork(ethAddress, networkName, db)
	if err != nil {
		FailOnError(c, err)
		return
	}

	im := models.NewHostedIPFSNetworkManager(db)
	apiURL, err := im.GetAPIURLByName(networkName)
	if err != nil {
		FailOnError(c, err)
		return
	}
	key := c.Param("key")
	manager, err := rtfs.Initialize("", apiURL)
	if err != nil {
		FailOnError(c, err)
		return
	}
	sizeInBytes, err := manager.GetObjectFileSizeInBytes(key)
	if err != nil {
		FailOnError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"object":        key,
		"size_in_bytes": sizeInBytes,
	})

}

// TODO: NEED TO FINISH
// NEED TO CALL QUEUE TO UPDATE DATABASE
// NEED TO TRIGGER CLUSTER UPLOAD AFTER
func AddFileToHostedIPFSNetwork(c *gin.Context) {
	ethAddress := GetAuthenticatedUserFromContext(c)

	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}
	mqURL, ok := c.MustGet("mq_conn_url").(string)
	if !ok {
		FailOnError(c, errors.New("failed to load rabbitmq"))
		return
	}
	err := CheckAccessForPrivateNetwork(ethAddress, networkName, db)
	if err != nil {
		FailOnError(c, err)
		return
	}

	holdTimeinMonths, exists := c.GetPostForm("hold_time")
	if !exists {
		FailNoExistPostForm(c, "hold_time")
		return
	}
	holdTimeInt, err := strconv.ParseInt(holdTimeinMonths, 10, 64)
	if err != nil {
		FailOnError(c, err)
		return
	}
	im := models.NewHostedIPFSNetworkManager(db)
	apiURL, err := im.GetAPIURLByName(networkName)
	if err != nil {
		FailOnError(c, err)
		return
	}

	ipfsManager, err := rtfs.Initialize("", apiURL)
	if err != nil {
		FailOnError(c, err)
		return
	}
	qm, err := queue.Initialize(queue.DatabaseFileAddQueue, mqURL)
	if err != nil {
		FailOnError(c, err)
		return
	}

	fmt.Println("fetching file")
	// fetch the file, and create a handler to interact with it
	fileHandler, err := c.FormFile("file")
	if err != nil {
		FailOnError(c, err)
		return
	}

	file, err := fileHandler.Open()
	if err != nil {
		FailOnError(c, err)
		return
	}
	resp, err := ipfsManager.Shell.Add(file)
	if err != nil {
		FailOnError(c, err)
		return
	}
	fmt.Println("file uploaded")
	dfa := queue.DatabaseFileAdd{
		Hash:             resp,
		HoldTimeInMonths: holdTimeInt,
		UploaderAddress:  ethAddress,
		NetworkName:      networkName,
	}
	fmt.Printf("+%v\n", dfa)
	err = qm.PublishMessage(dfa)
	if err != nil {
		FailOnError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status": resp,
	})
}

func IpfsPubSubPublishToHostedIPFSNetwork(c *gin.Context) {
	ethAddress := GetAuthenticatedUserFromContext(c)
	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}

	err := CheckAccessForPrivateNetwork(ethAddress, networkName, db)
	if err != nil {
		FailOnError(c, err)
	}

	im := models.NewHostedIPFSNetworkManager(db)
	apiURL, err := im.GetAPIURLByName(networkName)
	if err != nil {
		FailOnError(c, err)
		return
	}
	topic := c.Param("topic")
	message, present := c.GetPostForm("message")
	if !present {
		FailNoExistPostForm(c, "message")
		return
	}
	manager, err := rtfs.Initialize("", apiURL)
	if err != nil {
		FailOnError(c, err)
		return
	}
	err = manager.PublishPubSubMessage(topic, message)
	if err != nil {
		FailOnError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"topic":   topic,
		"message": message,
	})
}

func IpfsPubSubConsumeForHostedIPFSNetwork(c *gin.Context) {
	cC := c.Copy()

	ethAddress := GetAuthenticatedUserFromContext(cC)
	if ethAddress != AdminAddress {
		FailNotAuthorized(c, "unauthorized access to admin route")
		return
	}
	networkName, exists := cC.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}

	db, ok := cC.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}
	im := models.NewHostedIPFSNetworkManager(db)
	apiURL, err := im.GetAPIURLByName(networkName)
	if err != nil {
		FailOnError(c, err)
		return
	}
	topic := cC.Param("topic")

	go func() {
		manager, err := rtfs.Initialize("", apiURL)
		if err != nil {
			fmt.Println(err)
			return
		}
		manager.SubscribeToPubSubTopic(topic)
		manager.ConsumeSubscription(manager.PubSub)
	}()

	c.JSON(http.StatusOK, gin.H{"status": "consuming messages in background"})
}

// RemovePinFromLocalHostForHostedIPFSNetwork is used to remove a content hash from a private hosted ipfs network
func RemovePinFromLocalHostForHostedIPFSNetwork(c *gin.Context) {
	hash := c.Param("hash")
	ethAddress := GetAuthenticatedUserFromContext(c)
	if ethAddress != AdminAddress {
		FailNotAuthorized(c, "unauthorized access to admin route")
		return
	}
	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}
	rm := queue.IPFSPinRemoval{
		ContentHash: hash,
		NetworkName: networkName,
		EthAddress:  ethAddress,
	}
	// TODO:
	// change to send a message to the cluster to depin
	mqConnectionURL := c.MustGet("mq_conn_url").(string)
	qm, err := queue.Initialize(queue.IpfsPinRemovalQueue, mqConnectionURL)
	if err != nil {
		FailOnError(c, err)
		return
	}
	err = qm.PublishMessageWithExchange(rm, queue.PinRemovalExchange)
	if err != nil {
		FailOnError(c, err)
		return
	}
	// TODO:
	// add in appropriate rabbitmq processing to delete from database
	c.JSON(http.StatusOK, gin.H{
		"status": "pin removal sent to backend",
	})
}

func GetLocalPinsForHostedIPFSNetwork(c *gin.Context) {
	ethAddress := GetAuthenticatedUserFromContext(c)
	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}
	err := CheckAccessForPrivateNetwork(ethAddress, networkName, db)
	if err != nil {
		FailOnError(c, err)
		return
	}
	im := models.NewHostedIPFSNetworkManager(db)
	apiURL, err := im.GetAPIURLByName(networkName)
	if err != nil {
		FailOnError(c, err)
		return
	}
	// initialize a connection toe the local ipfs node
	manager, err := rtfs.Initialize("", apiURL)
	if err != nil {
		FailOnError(c, err)
		return
	}
	// get all the known local pins
	// WARNING: THIS COULD BE A VERY LARGE LIST
	pinInfo, err := manager.Shell.Pins()
	if err != nil {
		FailOnError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"pins": pinInfo})
}

func GetObjectStatForIpfsForHostedIPFSNetwork(c *gin.Context) {
	ethAddress := GetAuthenticatedUserFromContext(c)
	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}
	err := CheckAccessForPrivateNetwork(ethAddress, networkName, db)
	if err != nil {
		FailOnError(c, err)
		return
	}

	im := models.NewHostedIPFSNetworkManager(db)
	apiURL, err := im.GetAPIURLByName(networkName)
	if err != nil {
		FailOnError(c, err)
		return
	}
	key := c.Param("key")
	manager, err := rtfs.Initialize("", apiURL)
	if err != nil {
		FailOnError(c, err)
		return
	}
	stats, err := manager.ObjectStat(key)
	if err != nil {
		FailOnError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

func CheckLocalNodeForPinForHostedIPFSNetwork(c *gin.Context) {
	ethAddress := GetAuthenticatedUserFromContext(c)
	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}
	err := CheckAccessForPrivateNetwork(ethAddress, networkName, db)
	if err != nil {
		FailOnError(c, err)
		return
	}
	im := models.NewHostedIPFSNetworkManager(db)
	apiURL, err := im.GetAPIURLByName(networkName)
	if err != nil {
		FailOnError(c, err)
		return
	}
	hash := c.Param("hash")
	manager, err := rtfs.Initialize("", apiURL)
	if err != nil {
		FailOnError(c, err)
		return
	}
	present, err := manager.ParseLocalPinsForHash(hash)
	if err != nil {
		FailOnError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"present": present})
}

func PublishDetailedIPNSToHostedIPFSNetwork(c *gin.Context) {

	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}

	ethAddress := GetAuthenticatedUserFromContext(c)

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}
	mqURL, ok := c.MustGet("mq_conn_url").(string)
	if !ok {
		FailOnError(c, errors.New("failed to load rabbitmq"))
		return
	}
	err := CheckAccessForPrivateNetwork(ethAddress, networkName, db)
	if err != nil {
		FailOnError(c, err)
		return
	}

	um := models.NewUserManager(db)
	qm, err := queue.Initialize(queue.IpnsUpdateQueue, mqURL)
	if err != nil {
		FailOnError(c, err)
		return
	}
	hash, present := c.GetPostForm("hash")
	if !present {
		FailNoExistPostForm(c, "hash")
		return
	}
	lifetimeStr, present := c.GetPostForm("life_time")
	if !present {
		FailNoExistPostForm(c, "lifetime")
		return
	}
	ttlStr, present := c.GetPostForm("ttl")
	if !present {
		FailNoExistPostForm(c, "ttl")
		return
	}
	key, present := c.GetPostForm("key")
	if !present {
		FailNoExistPostForm(c, "key")
		return
	}
	resolveString, present := c.GetPostForm("resolve")
	if !present {
		FailNoExistPostForm(c, "resolve")
		return
	}

	ownsKey, err := um.CheckIfKeyOwnedByUser(ethAddress, key)
	if err != nil {
		FailOnError(c, err)
		return
	}

	if !ownsKey {
		FailOnError(c, errors.New("attempting to generate IPNS entry unowned key"))
		return
	}

	im := models.NewHostedIPFSNetworkManager(db)
	apiURL, err := im.GetAPIURLByName(networkName)
	if err != nil {
		FailOnError(c, err)
		return
	}
	manager, err := rtfs.Initialize("", apiURL)
	if err != nil {
		FailOnError(c, err)
		return
	}
	fmt.Println("creating key store manager")
	err = manager.CreateKeystoreManager()
	if err != nil {
		FailOnError(c, err)
		return
	}
	resolve, err := strconv.ParseBool(resolveString)
	if err != nil {
		FailOnError(c, err)
		return
	}
	lifetime, err := time.ParseDuration(lifetimeStr)
	if err != nil {
		FailOnError(c, err)
		return
	}
	ttl, err := time.ParseDuration(ttlStr)
	if err != nil {
		FailOnError(c, err)
		return
	}
	prePubTime := time.Now()
	keyID, err := um.GetKeyIDByName(ethAddress, key)
	fmt.Println("using key id of ", keyID)
	if err != nil {
		FailOnError(c, err)
		return
	}
	fmt.Println("publishing to IPNS")
	resp, err := manager.PublishToIPNSDetails(hash, key, lifetime, ttl, resolve)
	if err != nil {
		FailOnError(c, err)
		return
	}
	postPubTime := time.Now()
	timeDifference := postPubTime.Sub(prePubTime)

	ipnsManager := models.NewIPNSManager(db)
	ipnsEntry, err := ipnsManager.UpdateIPNSEntry(resp.Name, resp.Value, key, networkName, lifetime, ttl)
	if err != nil {
		FailOnError(c, err)
		return
	}
	ipnsUpdate := queue.IPNSUpdate{
		CID:         hash,
		IPNSHash:    resp.Name,
		LifeTime:    lifetime.String(),
		TTL:         ttl.String(),
		Key:         key,
		Resolve:     resolve,
		NetworkName: networkName,
	}
	err = qm.PublishMessage(ipnsUpdate)
	if err != nil {
		FailOnError(c, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"name":                   resp.Name,
		"value":                  resp.Value,
		"time_to_create_minutes": timeDifference.Minutes(),
		"ipns_entry_model":       ipnsEntry,
	})
}

func CreateHostedIPFSNetworkEntryInDatabase(c *gin.Context) {
	// lock down as admin route for now
	cC := c.Copy()
	ethAddress := GetAuthenticatedUserFromContext(cC)
	if ethAddress != AdminAddress {
		FailNotAuthorized(c, "unauthorized access")
		return
	}

	networkName, exists := cC.GetPostForm("network_name")
	if !exists {
		FailNoExist(c, "network_name post form does not exist")
		return
	}

	apiURL, exists := cC.GetPostForm("api_url")
	if !exists {
		FailNoExist(c, "api_url post form does not exist")
		return
	}

	swarmKey, exists := cC.GetPostForm("swarm_key")
	if !exists {
		FailNoExist(c, "swarm_key post form does not exist")
		return
	}

	bPeers, exists := cC.GetPostFormArray("bootstrap_peers")
	if !exists {
		FailNoExist(c, "boostrap_peers post form array does not exist")
		return
	}
	nodeAddresses, exists := cC.GetPostFormArray("local_node_addresses")
	if !exists {
		FailNoExist(c, "local_node_addresses post form array does not exist")
		return
	}
	users := cC.PostFormArray("users")
	var localNodeAddresses []string
	var bootstrapPeerAddresses []string

	if len(nodeAddresses) != len(bPeers) {
		FailOnError(c, errors.New("length of local_node_addresses and bootstrap_peers must be equal"))
		return
	}
	for k, v := range bPeers {
		addr, err := utils.GenerateMultiAddrFromString(v)
		if err != nil {
			FailOnError(c, err)
			return
		}
		valid, err := utils.ParseMultiAddrForIPFSPeer(addr)
		if err != nil {
			FailOnError(c, err)
			return
		}
		if !valid {
			FailOnError(c, errors.New(fmt.Sprintf("provided peer %s is not a valid bootstrap peer", addr)))
			return
		}
		addr, err = utils.GenerateMultiAddrFromString(nodeAddresses[k])
		if err != nil {
			FailOnError(c, err)
			return
		}
		valid, err = utils.ParseMultiAddrForIPFSPeer(addr)
		if err != nil {
			FailOnError(c, err)
			return
		}
		if !valid {
			FailOnError(c, errors.New(fmt.Sprintf("provided peer %s is not a valid ipfs peer", addr)))
			return
		}
		bootstrapPeerAddresses = append(bootstrapPeerAddresses, v)
		localNodeAddresses = append(localNodeAddresses, nodeAddresses[k])
	}
	// previously we were initializing like `var args map[string]*[]string` which was causing some issues.
	args := make(map[string][]string)
	args["local_node_peer_addresses"] = localNodeAddresses
	if len(bootstrapPeerAddresses) > 0 {
		args["bootstrap_peer_addresses"] = bootstrapPeerAddresses
	}
	db, ok := cC.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}
	manager := models.NewHostedIPFSNetworkManager(db)
	network, err := manager.CreateHostedPrivateNetwork(networkName, apiURL, swarmKey, args, users)
	if err != nil {
		FailOnError(c, err)
		return
	}
	um := models.NewUserManager(db)

	if len(users) > 0 {
		for _, v := range users {
			err := um.AddIPFSNetworkForUser(v, networkName)
			if err != nil {
				FailOnError(c, err)
				return
			}
		}
	} else {
		err := um.AddIPFSNetworkForUser(AdminAddress, networkName)
		if err != nil {
			FailOnError(c, err)
			return
		}
	}
	c.JSON(http.StatusCreated, gin.H{
		"network": network,
	})

}

func GetIPFSPrivateNetworkByName(c *gin.Context) {
	ethAddress := GetAuthenticatedUserFromContext(c)
	if ethAddress != AdminAddress {
		FailNotAuthorized(c, "unauthorized access")
		return
	}
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}

	netName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}
	manager := models.NewHostedIPFSNetworkManager(db)
	net, err := manager.GetNetworkByName(netName)
	if err != nil {
		FailOnError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"network": net,
	})
}

func GetAuthorizedPrivateNetworks(c *gin.Context) {
	ethAddress := GetAuthenticatedUserFromContext(c)
	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}

	um := models.NewUserManager(db)
	networks, err := um.GetPrivateIPFSNetworksForUser(ethAddress)
	if err != nil {
		FailOnError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"networks": networks,
	})
}

func GetUploadsByNetworkName(c *gin.Context) {
	ethAddress := GetAuthenticatedUserFromContext(c)

	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}

	err := CheckAccessForPrivateNetwork(ethAddress, networkName, db)
	if err != nil {
		FailOnError(c, err)
		return
	}

	um := models.NewUploadManager(db)
	uploads, err := um.FindUploadsByNetwork(networkName)
	if err != nil {
		FailOnError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"uploads": uploads,
	})
}

// DownloadContentHash is used to download a particular content hash from the network
//
func DownloadContentHashForPrivateNetwork(c *gin.Context) {
	networkName, exists := c.GetPostForm("network_name")
	if !exists {
		FailNoExistPostForm(c, "network_name")
		return
	}

	var contentType string
	// fetch the specified content type from the user
	contentType, exists = c.GetPostForm("content_type")
	// if not specified, provide a default
	if !exists {
		contentType = "application/octet-stream"
	}

	// get any extra headers the user might want
	exHeaders := c.PostFormArray("extra_headers")

	ethAddress := GetAuthenticatedUserFromContext(c)

	db, ok := c.MustGet("db").(*gorm.DB)
	if !ok {
		FailedToLoadDatabase(c)
		return
	}
	err := CheckAccessForPrivateNetwork(ethAddress, networkName, db)
	if err != nil {
		FailOnError(c, err)
		return
	}
	im := models.NewHostedIPFSNetworkManager(db)
	apiURL, err := im.GetAPIURLByName(networkName)
	if err != nil {
		FailOnError(c, err)
		return
	}
	// get the content hash that is to be downloaded
	contentHash := c.Param("hash")

	// initialize our connection to IPFS
	manager, err := rtfs.Initialize("", apiURL)
	if err != nil {
		FailOnError(c, err)
		return
	}
	// read the contents of the file
	reader, err := manager.Shell.Cat(contentHash)
	if err != nil {
		FailOnError(c, err)
		return
	}
	// get the size of hte file in bytes
	sizeInBytes, err := manager.GetObjectFileSizeInBytes(contentHash)
	if err != nil {
		FailOnError(c, err)
		return
	}
	// parse extra headers if there are any
	extraHeaders := make(map[string]string)
	var header string
	var value string
	// only process if there is actual data to process
	// this will always be admin locked
	if len(exHeaders) > 0 && ethAddress == AdminAddress {
		// the array must be of equal length, as a header has two parts
		// the name of the header, and its value
		// this expects the user to have properly formatted the headers
		// we will need to restrict the headers that we process so we don't
		// open ourselves up to being attacked
		if len(exHeaders)%2 != 0 {
			FailOnError(c, errors.New("extra_headers post form is not even in length"))
			return
		}
		// parse through the available headers
		for i := 1; i < len(exHeaders)-1; i += 2 {
			// retrieve header name
			header = exHeaders[i-1]
			// retrieve header value
			value = exHeaders[i]
			// store data
			extraHeaders[header] = value
		}
	}
	// send them the file
	c.DataFromReader(200, int64(sizeInBytes), contentType, reader, extraHeaders)
}
