package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	auth "github.com/AkinAD/basedCode/auth"
	shop "github.com/AkinAD/basedCode/shop"
	user "github.com/AkinAD/basedCode/user"
	cognito "github.com/aws/aws-sdk-go/service/cognitoidentityprovider"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"github.com/aws/aws-sdk-go/aws"
)

var (
	userSrv user.UserService
	shopSrv shop.ShopService
	//storeSrv db.DbService
	// authSrv auth.AuthService

	port               string
	connString         string
	userPoolID         string
	awsRegion          string
	awsID              string
	awsSecret          string
	cognitoAppClientID string
)

func main() {
	// Logging to a file.
	f, _ := os.Create("smartshopper.log")
	defer f.Close()
	// Use the following code if you need to write the logs to file and console at the same time.
	gin.DefaultWriter = io.MultiWriter(f, os.Stdout)
	log.SetPrefix("[DEBUG] ")
	log.SetOutput(io.MultiWriter(f, os.Stdout))

	router := gin.Default()

	router.Use(corsMiddleware)

	userSrv = user.NewService(awsRegion, awsID, awsSecret, connString)
	shopSrv = shop.NewService(connString)
	// authSrv = auth.NewService()

	// heartbeat
	router.GET("/", homeHandler)
	router.GET("/heartbeat", auth.AuthMiddleware(awsRegion, userPoolID, []string{"user", "employee", "manager", "admin"}), heartbeat)

	//login
	router.POST("/login", login)

	//all account types
	router.GET("/account", auth.AuthMiddleware(awsRegion, userPoolID, []string{"user", "employee", "manager", "admin"}), getAccount)
	router.GET("/account/:user", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), getAccountByUsername)
	router.PUT("/account/:id", auth.AuthMiddleware(awsRegion, userPoolID, []string{"user", "employee", "manager", "admin"}), updateAccount)
	//router.GET("/account/:id", auth.AuthMiddleware(awsRegion, userPoolID, []string{"user", "employee", "manager", "admin"}), getProfile)

	//users
	router.GET("/user", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), getGroupUser)

	//employees
	router.GET("/employee", auth.AuthMiddleware(awsRegion, userPoolID, []string{"employee", "manager", "admin"}), getGroupEmployee)
	router.POST("/employee", auth.AuthMiddleware(awsRegion, userPoolID, []string{"manager", "admin"}), createEmployee)
	// router.PUT("/employee", auth.AuthMiddleware(cognitoRegion, userPoolID, []string{"employee", "manager", "admin"}), updateEmployee)
	// router.DELETE("/employee", auth.AuthMiddleware(cognitoRegion, userPoolID, []string{manager", "admin"}), deleteEmployee)
	router.DELETE("/employee", auth.AuthMiddleware(awsRegion, userPoolID, []string{"manager", "admin"}), deleteFromAdmin)

	//managers
	router.GET("/manager", auth.AuthMiddleware(awsRegion, userPoolID, []string{"manager", "admin"}), getGroupManager)
	router.POST("/manager", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), promoteToManager)
	// router.DELETE("manager/:id", deleteManager)
	router.DELETE("/manager", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin", "manager"}), deleteFromAdmin)

	//admin
	router.GET("/admin", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), getGroupAdmin)
	router.POST("/admin", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), promoteToAdmin)
	router.DELETE("/admin", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), deleteFromAdmin)

	//item
	router.GET("/item", getItems) //?storeID= to get the shops/stock for a specific store
	router.GET("/item/:id", getItem)
	router.POST("/item", auth.AuthMiddleware(awsRegion, userPoolID, []string{"employee", "manager", "admin"}), createItem)
	router.PUT("/item/:id", auth.AuthMiddleware(awsRegion, userPoolID, []string{"employee", "manager", "admin"}), updateItem)
	router.DELETE("/item/:id", auth.AuthMiddleware(awsRegion, userPoolID, []string{"employee", "manager", "admin"}), deleteItem)

	//store
	router.GET("/store", getStores)
	router.GET("/store/:id", getStore) //return store + stock
	router.POST("/store", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), createStore)
	router.PUT("/store", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), updateStore)
	router.DELETE("/store/:id", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), deleteStore)

	//stock
	router.POST("/stock", auth.AuthMiddleware(awsRegion, userPoolID, []string{"employee", "manager", "admin"}), createStock)
	router.PUT("/stock", auth.AuthMiddleware(awsRegion, userPoolID, []string{"employee", "manager", "admin"}), editStock)
	router.DELETE("/stock/:store/:item", auth.AuthMiddleware(awsRegion, userPoolID, []string{"employee", "manager", "admin"}), deleteStock)

	//item
	router.GET("/category", getCategories)
	router.POST("/category", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), createCategory)
	router.PUT("/category", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), updateCategory)
	router.DELETE("/category/:id", auth.AuthMiddleware(awsRegion, userPoolID, []string{"admin"}), deleteCategory)

	if port == "443" {
		router.RunTLS(":"+port, "./certs/smartshopper_certificate.cer", "./certs/smartshopper_key.key")
	} else {
		router.Run(":" + port)
	}
}

func init() {
	connString = initPostgres()
	port = defaulter("PORT", "8081")
	awsRegion = defaulter("AWS_REGION", "us-east-2")
	awsID = defaulter("AWS_ID", "")
	awsSecret = defaulter("AWS_SECRET", "")
	userPoolID = defaulter("COGNITO_USER_POOL_ID", "")
	cognitoAppClientID = defaulter("COGNITO_APP_CLIENT_ID", "")
}

func initPostgres() string {
	// PGHost := defaulter("PG_HOST", "localhost")
	PGHost := defaulter("PG_HOST", "localhost")
	PGPort := defaulter("PG_PORT", "5432")
	PGUser := defaulter("PG_USER", "postgres")
	PGPass := defaulter("PG_PASS", "")
	PGName := defaulter("PG_NAME", "postgres")
	// PGSSL := defaulter("PG_SSLMODE", "disable")

	return fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=disable",
		PGUser, PGPass, PGName, PGHost, PGPort)
}

func defaulter(envName, defaultValue string) string {
	input := os.Getenv(envName)
	if len(input) == 0 {
		input = defaultValue
	}

	return input
}

var corsMiddleware = cors.New(cors.Config{
	// AllowOrigins:     []string{"https://wheypal.com", "http://localhost:8080"},
	AllowOrigins: []string{"*"},
	AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
	// AllowMethods:     []string{"*"},
	AllowHeaders:     []string{"Authorization", "Origin", "Content-Length", "Content-Type"},
	AllowCredentials: true,
	MaxAge:           12 * time.Hour,
})

func homeHandler(c *gin.Context) {
	c.JSON(
		200,
		gin.H{"message": "whats poppin"},
	)
}

func heartbeat(c *gin.Context) {
	groups, _ := c.Get("groups")

	c.JSON(
		200,
		gin.H{"groups": groups},
	)
}

func login(c *gin.Context) {
	type LoginRequest struct {
		Username string
		Password string
	}
	var login LoginRequest
	err := c.ShouldBind(&login)
	if err != nil {
		c.JSON(401, err)
	}

	authParams := make(map[string]*string)
	authParams["USERNAME"] = aws.String(login.Username)
	authParams["PASSWORD"] = aws.String(login.Password)

	input := &cognito.InitiateAuthInput{
		AuthFlow:       aws.String("USER_PASSWORD_AUTH"),
		AuthParameters: authParams,
		ClientId:       aws.String(cognitoAppClientID),
	}

	res, err := userSrv.Login(input)
	if err != nil {
		c.JSON(401, err)
	}

	c.JSON(200, res)
}

func getAccount(c *gin.Context) {

	username := c.GetString("username")
	if username == "" {
		c.AbortWithError(500, errors.New("Could not get username from token"))
		return
	}
	// usernameStr := username.(string)

	log.Printf("[Gateway] [GetAccount] %s\n", username)

	input := &cognito.AdminGetUserInput{
		UserPoolId: aws.String(userPoolID),
		Username:   aws.String(username),
	}

	_, err := userSrv.GetUser(input)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	//no longer printing out the default aws cognito response (for normalization sake)
	//c.JSON(200, resp)
	getProfile(c, username)

}

func getAccountByUsername(c *gin.Context) {

	username := c.Param("user")

	log.Printf("[Gateway] [GetAccountByUsername] %s\n", username)

	input := &cognito.AdminGetUserInput{
		UserPoolId: aws.String(userPoolID),
		Username:   aws.String(username),
	}

	_, err := userSrv.GetUser(input)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}
	//no longer printing out the default aws cognito response (for normalization sake)
	//c.JSON(200, &resp)
	getProfile(c, username)
}

func updateAccount(c *gin.Context) {
	var user *user.User
	err := c.ShouldBind(&user)
	if err != nil {
		c.JSON(401, err)
	}

	// grab the username and connect to the userDB to find them
	// then update the db with the preferred location
	//err = shopSrv.db.Table("account").Where("username = ?", update.username).Update("storeID", update.preferredStore)

	resp, err := userSrv.UpdateProfile(user)
	if err != nil {
		c.JSON(401, err)
	}
	c.JSON(200, &resp)
}

func getProfile(c *gin.Context, username string) {

	resp, err := userSrv.GetProfile(username)
	if err != nil {
		c.JSON(401, err)
		return
	}
	c.JSON(200, resp)
}

func getGroupUser(c *gin.Context) {
	getGroup(c, "user")
}

func getGroupEmployee(c *gin.Context) {
	getGroup(c, "employee")
}

func getGroupManager(c *gin.Context) {
	getGroup(c, "manager")
}

func getGroupAdmin(c *gin.Context) {
	getGroup(c, "admin")
}

func getGroup(c *gin.Context, group string) {
	input := &cognito.ListUsersInGroupInput{
		GroupName: aws.String(group),
		//	NextToken:  aws.String("1"),
		UserPoolId: aws.String(userPoolID),
	}

	resp, err := userSrv.ListUsersInGroup(input)
	if err != nil {
		c.JSON(500, err)
	}

	c.JSON(200, resp)
}

func deleteFromAdmin(c *gin.Context) {
	//grab username to be deleted
	var userToBeDeleted struct {
		Username string `json:"username"`
	}
	err := c.ShouldBind(&userToBeDeleted)
	if err != nil {
		c.JSON(401, err)
	}
	fmt.Println(userToBeDeleted.Username)

	//grab current user
	currentUser := c.GetString("username")
	if currentUser == "" {
		c.AbortWithError(500, errors.New("Could not get username from token"))
		return
	}

	//check permissions of current user to see if they are admin/manager
	input := &cognito.AdminListGroupsForUserInput{
		UserPoolId: aws.String(userPoolID),
		Username:   aws.String(currentUser),
	}
	userPool, errCheck := userSrv.ListGroupsForUser(input)

	if errCheck != nil {
		c.AbortWithError(500, errCheck)
		return
	}
	adminCheck := false
	managerCheck := false
	employeeCheck := false

	length := len(userPool.Groups)
	for i := 0; i < length; i++ {
		if *userPool.Groups[i].GroupName == "manager" {
			managerCheck = true
		}
		if *userPool.Groups[i].GroupName == "admin" {
			adminCheck = true
		}
	}
	//check permissions of userToBeDeleted to see if they are an employee
	input2 := &cognito.AdminListGroupsForUserInput{
		UserPoolId: aws.String(userPoolID),
		Username:   aws.String(userToBeDeleted.Username),
	}

	userPoolEmployee, errCheck2 := userSrv.ListGroupsForUser(input2)
	if errCheck2 != nil {
		c.AbortWithError(500, errCheck2)
		return
	}

	length = len(userPoolEmployee.Groups)
	for i := 0; i < length; i++ {
		if *userPoolEmployee.Groups[i].GroupName == "employee" {
			employeeCheck = true
			fmt.Println(*userPoolEmployee.Groups[i].GroupName)
		}
	}

	if adminCheck == true {
		fmt.Println("You are a Admin")

		//can delete anyone if you are an admin
		input := &cognito.AdminDeleteUserInput{
			Username:   aws.String(userToBeDeleted.Username),
			UserPoolId: aws.String(userPoolID),
		}

		resp, err := userSrv.DeleteUser(input)
		if err != nil {
			c.JSON(500, err)
		}

		status, err2 := userSrv.DeleteProfile(userToBeDeleted.Username)
		if err2 != nil {
			c.JSON(500, err2)
		}
		fmt.Println(status)
		c.JSON(200, resp)

	} else if managerCheck == true {
		fmt.Println("You are a Manager")
		//can only delete employees in your store if you are a manager
		managerInfo, managerErr := userSrv.GetProfile(currentUser)
		if managerErr != nil {
			c.JSON(500, managerErr)
		}
		//check if the user to be deleted is an employee
		if employeeCheck == true {
			//get employee's profile to compare storeIDs with Manager
			employeeInfo, employeeErr := userSrv.GetProfile(userToBeDeleted.Username)
			if employeeErr != nil {
				c.JSON(500, employeeErr)
			}

			//compare the storeIDs
			if managerInfo.StoreID != employeeInfo.StoreID {
				c.AbortWithError(500, errors.New("The employee's StoreID does not match the Manager's Store ID"))
				return
			}

			//delete the user
			input := &cognito.AdminDeleteUserInput{
				Username:   aws.String(userToBeDeleted.Username),
				UserPoolId: aws.String(userPoolID),
			}

			resp, err := userSrv.DeleteUser(input)
			if err != nil {
				c.JSON(500, err)
			}

			status, err2 := userSrv.DeleteProfile(userToBeDeleted.Username)
			if err2 != nil {
				c.JSON(500, err2)
			}
			fmt.Println(status)

			c.JSON(200, resp)
		} else {
			c.AbortWithError(500, errors.New("User to be deleted it not an employee"))
		}

	} else {
		c.AbortWithError(500, errors.New("Current user is not a Manager or Admin"))
	}

}

func createEmployee(c *gin.Context) {
	//binded variables change depending on what is being sent from front-end
	type CreateEmployeeInput struct {
		Username  string `json:"username"`
		Email     string `json:"email"`
		StoreID   int    `json:"storeid"`
		FirstName string `json:"firstname"`
		LastName  string `json:"lastname"`
	}
	var input CreateEmployeeInput

	err := c.ShouldBind(&input)
	if err != nil {
		c.JSON(500, err)
	}
	payload := &cognito.AdminCreateUserInput{
		DesiredDeliveryMediums: []*string{aws.String("EMAIL")},
		// ForceAliasCreation:     aws.Bool(true),
		UserAttributes: []*cognito.AttributeType{&cognito.AttributeType{Name: aws.String(cognito.UsernameAttributeTypeEmail), Value: aws.String(input.Email)}},
		UserPoolId:     aws.String(userPoolID),
		Username:       aws.String(input.Username),
	}

	//check to see if the current user is admin or manager
	//grab current user
	currentUser := c.GetString("username")
	if currentUser == "" {
		c.AbortWithError(500, errors.New("Could not get username from token"))
		return
	}

	//check permissions of current user to see if they are admin/manager
	input2 := &cognito.AdminListGroupsForUserInput{
		UserPoolId: aws.String(userPoolID),
		Username:   aws.String(currentUser),
	}
	userPool, errCheck := userSrv.ListGroupsForUser(input2)

	if errCheck != nil {
		c.AbortWithError(500, errCheck)
		return
	}
	adminCheck := false
	managerCheck := false

	length := len(userPool.Groups)
	for i := 0; i < length; i++ {
		if *userPool.Groups[i].GroupName == "manager" {
			managerCheck = true
		}
		if *userPool.Groups[i].GroupName == "admin" {
			adminCheck = true
		}
	}

	if adminCheck == true {
		fmt.Println("You are a Admin")
		//create employee
		resp, err := userSrv.CreateEmployee(payload)
		if err != nil {
			c.JSON(500, err)
		}
		c.JSON(200, resp)

		//add them to employee group
		input2 := &cognito.AdminAddUserToGroupInput{
			GroupName:  aws.String("employee"),
			UserPoolId: aws.String(userPoolID),
			Username:   aws.String(input.Username),
		}
		_, err2 := userSrv.AddUserToGroup(input2)
		if err2 != nil {
			c.JSON(500, err2)
		}

		//create their profile in userDB
		err3 := userSrv.CreateProfile(input.Username, input.StoreID, input.FirstName, input.LastName, input.Email)
		if err3 != nil {
			c.JSON(500, err3)
		}
	} else if managerCheck == true {

		//get manager's profile to get their storeID, which is used for creating the employee
		managerUsername := c.GetString("username")
		if managerUsername == "" {
			c.AbortWithError(500, errors.New("Could not get managerUsername from token"))
			return
		}
		managerInfo, managerErr := userSrv.GetProfile(managerUsername)
		if managerErr != nil {
			c.JSON(500, managerErr)
		}
		//fmt.Println(managerInfo.StoreID)
		if managerInfo.StoreID != input.StoreID {
			c.AbortWithError(500, errors.New("The employee's StoreID does not match the Manager's Store ID"))
			return
		}

		//create employee
		resp, err := userSrv.CreateEmployee(payload)
		if err != nil {
			c.JSON(500, err)
		}
		c.JSON(200, resp)

		//add them to employee group
		input2 := &cognito.AdminAddUserToGroupInput{
			GroupName:  aws.String("employee"),
			UserPoolId: aws.String(userPoolID),
			Username:   aws.String(input.Username),
		}
		_, err2 := userSrv.AddUserToGroup(input2)
		if err2 != nil {
			c.JSON(500, err2)
		}

		//create their profile in userDB
		err3 := userSrv.CreateProfile(input.Username, managerInfo.StoreID, input.FirstName, input.LastName, input.Email)
		if err3 != nil {
			c.JSON(500, err3)
		}

	}

}

func promoteToManager(c *gin.Context) {
	// need user to be promoted, changing their UserPoolId to 2 (manager)
	promoteTo(c, "manager")

}

func promoteToAdmin(c *gin.Context) {
	// need user to be promoted, changing their UserPoolId to 1 (admin)
	promoteTo(c, "admin")
}

func promoteTo(c *gin.Context, group string) {
	type userInfo struct {
		Username string `json:"username"`
	}
	var userInputInfo userInfo

	err := c.ShouldBind(&userInputInfo)
	if err != nil {
		c.JSON(500, err)
	}
	//c.JSON(200, gin.H{"username": userInputInfo.Username})

	input := &cognito.AdminAddUserToGroupInput{
		GroupName:  aws.String(group),
		UserPoolId: aws.String(userPoolID),
		Username:   aws.String(userInputInfo.Username),
	}
	//c.JSON(200, gin.H{"input": input})

	resp, err := userSrv.AddUserToGroup(input)
	if err != nil {
		c.JSON(500, err)
	}

	c.JSON(200, resp)
}

func getItems(c *gin.Context) {
	items, err := shopSrv.GetItems()

	if err != nil {
		c.JSON(500, err)
	}

	c.JSON(200, &items)
}

func getItem(c *gin.Context) {
	idString := c.Param("id")
	id, err := strconv.Atoi(idString)

	if err != nil {
		c.JSON(500, err)
	}

	log.Printf("[Main] [GetItem] %v", id)
	resp, err := shopSrv.GetItem(id)

	if err != nil {
		c.JSON(500, err)
	}

	c.JSON(200, &resp)
}

func createItem(c *gin.Context) {
	var request *shop.Item
	err := c.ShouldBind(&request)
	if err != nil {
		c.AbortWithError(502, err)
	}
	fmt.Println("createItem lots of boba")
	// if POSTMAN request body doesn't have itemID then &resp is null
	resp, err := shopSrv.CreateItem(request)
	if err != nil {
		c.AbortWithError(502, err)
	}

	c.JSON(200, &resp)
}

func updateItem(c *gin.Context) {
	var request *shop.Item
	err := c.ShouldBind(&request)
	if err != nil {
		c.AbortWithError(502, err)
	}

	idString := c.Param("id")
	id, err := strconv.Atoi(idString)

	request.ItemID = id

	if err != nil {
		c.JSON(500, err)
	}

	fmt.Println("updateItem request")
	fmt.Printf("%+v\n", request)
	// if POSTMAN request body doesn't have itemID then &resp is null
	resp, err := shopSrv.UpdateItem(request)
	if err != nil {
		c.AbortWithError(502, err)
	}

	c.JSON(200, &resp)
}

func deleteItem(c *gin.Context) {
	idString := c.Param("id")
	id, err := strconv.Atoi(idString)

	if err != nil {
		c.JSON(500, err)
	}

	deleteResult, err := shopSrv.DeleteItem(id)

	if err != nil {
		c.JSON(500, err)
	}

	c.JSON(200, &deleteResult)
}

func getStores(c *gin.Context) {
	resp, err := shopSrv.GetStores()

	if err != nil {
		c.JSON(500, err)
	}

	c.JSON(200, &resp)
}

func getStore(c *gin.Context) {
	idString := c.Param("id")
	id, err := strconv.Atoi(idString)

	if err != nil {
		c.JSON(500, err)
	}

	resp, err := shopSrv.GetStore(id)

	if err != nil {
		c.JSON(500, err)
	}

	c.JSON(200, &resp)
}

func createStore(c *gin.Context) {
	var request *shop.Store
	err := c.ShouldBind(&request)
	if err != nil {
		c.AbortWithError(502, err)
	}

	resp, err := shopSrv.CreateStore(request)
	if err != nil {
		c.AbortWithError(502, err)
	}

	c.JSON(200, &resp)
}

func updateStore(c *gin.Context) {
	var request *shop.Store
	err := c.ShouldBind(&request)
	if err != nil {
		c.AbortWithError(502, err)
	}

	resp, err := shopSrv.UpdateStore(request)
	if err != nil {
		c.AbortWithError(502, err)
	}

	c.JSON(200, &resp)
}

func deleteStore(c *gin.Context) {
	id := c.Param("id")
	storeID, err := strconv.Atoi(id)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	resp, err := shopSrv.DeleteStore(storeID)

	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	c.JSON(200, &resp)
}

func createStock(c *gin.Context) {
	//get user group from params thanks to auth wrapper

	var request *shop.StockRequest

	err := c.ShouldBind(&request)
	if err != nil {
		c.JSON(500, err)
		return
	}

	if !isAdmin(c) {
		username := c.MustGet("username").(string)
		user, err := userSrv.GetProfile(username) //change to user's shop
		log.Printf("[Gateway] [CreateStock] [GetUser] %s - %v\n", username, user)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		// request.StoreID = user.StoreID
		request.StoreID = 2 //@CHUN WAH THE ABOVE DOES NOT WORK
	}

	log.Printf("[Main] [CreateStock] %v", request)

	resp, err := shopSrv.CreateStock(request)
	if err != nil {
		c.JSON(500, err)
		return
	}

	c.JSON(200, resp)
}

func editStock(c *gin.Context) {
	var request *shop.StockRequest

	err := c.ShouldBind(&request)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	if !isAdmin(c) {
		username := c.MustGet("username").(string)
		user, err := userSrv.GetProfile(username) //change to user's shop
		log.Printf("[Gateway] [EditStock] [GetUser] %s - %v\n", username, user)
		if err != nil {
			c.AbortWithError(500, err)
			return
		}
		// request.StoreID = user.StoreID
		request.StoreID = 2 //@CHUN WAH THE ABOVE DOES NOT WORK
	}

	resp, err := shopSrv.UpdateStock(request)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	c.JSON(200, resp)
}

func deleteStock(c *gin.Context) {
	// type Request struct {
	// 	StoreID int `json:"storeID"`
	// 	ItemID  int `json:"itemID"`
	// }
	// var request Request

	// err := c.ShouldBind(&request)

	storeParam := c.Param("store")
	itemParam := c.Param("item")

	storeID, err := strconv.Atoi(storeParam)
	if err != nil {
		c.JSON(500, err)
	}

	itemID, err := strconv.Atoi(itemParam)
	if err != nil {
		c.JSON(500, err)
	}

	if !isAdmin(c) {
		username := c.MustGet("username").(string)
		user, err := userSrv.GetProfile(username) //change to user's shop
		if err != nil {
			c.JSON(500, err)
		}
		storeID = user.StoreID
	}

	resp, err := shopSrv.DeleteStock(storeID, itemID)
	if err != nil {
		c.JSON(500, err)
	}

	c.JSON(200, resp)
}

func isAdmin(c *gin.Context) bool {
	groups := c.GetStringSlice("groups")
	// log.Printf("[Main] [isAdmin] %s - %v", c.GetString("username"), groups)
	for _, group := range groups {
		if group == "admin" {
			return true
		}
	}
	return false
}

func getCategories(c *gin.Context) {
	resp, err := shopSrv.GetCategories()

	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	c.JSON(200, &resp)
}

func createCategory(c *gin.Context) {
	type Request struct {
		Name string `json:"name"`
	}
	var request *Request
	err := c.ShouldBind(&request)
	if err != nil {
		c.AbortWithError(502, err)
		return
	}

	resp, err := shopSrv.CreateCategory(request.Name)
	if err != nil {
		c.AbortWithError(502, err)
		return
	}

	c.JSON(200, &resp)
}

func updateCategory(c *gin.Context) {
	var request *shop.Category
	err := c.ShouldBind(&request)
	if err != nil {
		c.AbortWithError(502, err)
		return
	}

	log.Printf("[Main] [GetItem] %v", request)
	resp, err := shopSrv.UpdateCategory(request)
	if err != nil {
		c.AbortWithError(502, err)
		return
	}

	c.JSON(200, &resp)
}

func deleteCategory(c *gin.Context) {
	id := c.Param("id")
	storeID, err := strconv.Atoi(id)
	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	resp, err := shopSrv.DeleteCategory(storeID)

	if err != nil {
		c.AbortWithError(500, err)
		return
	}

	c.JSON(200, &resp)
}
