package controller

import (
	"gsc/middleware"
	"gsc/model"
	"gsc/utils"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/midtrans/midtrans-go"
	"github.com/midtrans/midtrans-go/coreapi"
	"gorm.io/gorm"
)

func CreditStore(db *gorm.DB, q *gin.Engine) {
	r := q.Group("/api/user/credit-store")
	// get all
	r.GET("/all", middleware.Authorization(), func(c *gin.Context) {
		var scores []model.CreditStore
		if err := db.Find(&scores).Error; err != nil {
			log.Println("di sini error mencari data")
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		utils.HttpRespSuccess(c, http.StatusOK, "All credit store", scores)
	})

	// view cart
	r.GET("/view-cart", middleware.Authorization(), func(c *gin.Context) {
		var total int
		var totalPoints int

		ID, _ := c.Get("id")
		userID, ok := ID.(uuid.UUID)
		if !ok {
			utils.HttpRespFailed(c, http.StatusNotFound, "User not found")
			return
		}

		var cart []model.CreditStoreWallet
		if err := db.Where("user_id = ?", userID).Find(&cart).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		for _, v := range cart {
			total += v.Price
			totalPoints += v.Points
		}

		utils.HttpRespSuccess(c, http.StatusOK, "View cart", gin.H{
			"total":       total,
			"totalPoints": totalPoints,
			"cart":        cart,
		})
	})

	// add to cart
	r.POST("/add-to-cart", middleware.Authorization(), func(c *gin.Context) {
		var input model.CreditStoreWalletInput
		if err := c.BindJSON(&input); err != nil {
			utils.HttpRespFailed(c, http.StatusUnprocessableEntity, err.Error())
			return
		}

		var credit model.CreditStore
		if err := db.Where("id = ?", input.ID).First(&credit).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		ID, _ := c.Get("id")
		userID, ok := ID.(uuid.UUID)
		if !ok {
			utils.HttpRespFailed(c, http.StatusNotFound, "User not found")
			return
		}

		parsedID, err := strconv.ParseUint(strconv.Itoa(input.ID), 10, 0)
		if err != nil {
			utils.HttpRespFailed(c, http.StatusUnprocessableEntity, err.Error())
			return
		}

		addToCart := model.CreditStoreWallet{
			UserID:        userID,
			CreditStoreID: uint(parsedID),
			Points:        credit.Points,
			Price:         credit.Price,
			Quantity:      1,
		}

		// handle error if user already add to cart
		var isExist model.CreditStoreWallet
		if err := db.Where("user_id = ? ", userID).Where("credit_store_id = ?", input.ID).First(&isExist).Error; err == nil {
			log.Println("sudah ada di cart")
			// update
			isExist.Points += credit.Points
			isExist.Price += credit.Price
			isExist.Quantity += 1
			if err := db.Where("user_id = ?", userID).Where("credit_store_id = ?", input.ID).Save(&isExist).Error; err != nil {
				log.Println("error update")
				utils.HttpRespFailed(c, http.StatusBadGateway, err.Error())
				return
			}

			utils.HttpRespSuccess(c, http.StatusOK, "added to cart", isExist)
			return
		}

		if err := db.Create(&addToCart).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		utils.HttpRespSuccess(c, http.StatusOK, "Add to cart", addToCart)
	})

	// add 1 amount by id
	r.POST("/add-amount/:itemID", middleware.Authorization(), func(c *gin.Context) {
		itemID := c.Param("itemID")

		var credit model.CreditStore
		if err := db.Where("id = ?", itemID).First(&credit).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		ID, _ := c.Get("id")
		userID, ok := ID.(uuid.UUID)
		if !ok {
			utils.HttpRespFailed(c, http.StatusNotFound, "User not found")
			return
		}

		var updated model.CreditStoreWallet
		if err := db.Where("credit_store_id = ?", itemID).Where("user_id = ?", userID).First(&updated).Error; err != nil {
			// utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			// return

			// its not in cart yet
			addToCart := model.CreditStoreWallet{
				UserID:        userID,
				CreditStoreID: credit.ID,
				Points:        credit.Points,
				Price:         credit.Price,
				Quantity:      1,
			}

			if err := db.Create(&addToCart).Error; err != nil {
				utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
				return
			}

			utils.HttpRespSuccess(c, http.StatusOK, "added new item", addToCart)
			return
		}

		updated.Points += credit.Points
		updated.Price += credit.Price
		updated.Quantity += 1

		// check if exist
		var isExist model.CreditStoreWallet
		if err := db.Where("user_id = ?", userID).Where("credit_store_id = ?", credit.ID).First(&isExist).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusBadRequest, err.Error())
			return
		}

		if err := db.Where("user_id = ?", userID).Where("credit_store_id = ?", credit.ID).Save(&updated).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		utils.HttpRespSuccess(c, http.StatusOK, "Added 1 amount", updated)
	})

	// remove 1 amount by id
	r.POST("/remove-amount/:itemID", middleware.Authorization(), func(c *gin.Context) {
		itemID := c.Param("itemID")

		var credit model.CreditStore
		if err := db.Where("id = ?", itemID).First(&credit).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		ID, _ := c.Get("id")
		userID, ok := ID.(uuid.UUID)
		if !ok {
			utils.HttpRespFailed(c, http.StatusNotFound, "User not found")
			return
		}

		var updated model.CreditStoreWallet
		if err := db.Where("credit_store_id = ?", itemID).Where("user_id = ?", userID).First(&updated).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		updated.Points -= credit.Points
		updated.Price -= credit.Price
		updated.Quantity -= 1

		// check if exist
		var isExist model.CreditStoreWallet
		if err := db.Where("user_id = ?", userID).Where("credit_store_id = ?", credit.ID).First(&isExist).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusBadRequest, err.Error())
			return
		}

		if updated.Quantity == 0 {
			if err := db.Where("user_id = ?", userID).Where("credit_store_id = ?", credit.ID).Delete(&updated).Error; err != nil {
				utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
				return
			}

			utils.HttpRespSuccess(c, http.StatusOK, "Removed", updated)
			return
		}

		if err := db.Where("user_id = ?", userID).Where("credit_store_id = ?", credit.ID).Save(&updated).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		utils.HttpRespSuccess(c, http.StatusOK, "Removed 1 amount", updated)
	})

	// remove item from cart
	r.DELETE("/remove-from-cart", middleware.Authorization(), func(c *gin.Context) {
		var input model.CreditStoreWalletInput
		if err := c.BindJSON(&input); err != nil {
			utils.HttpRespFailed(c, http.StatusUnprocessableEntity, err.Error())
			return
		}

		ID, _ := c.Get("id")
		userID, ok := ID.(uuid.UUID)
		if !ok {
			utils.HttpRespFailed(c, http.StatusNotFound, "User not found")
			return
		}

		var cart model.CreditStoreWallet
		if err := db.Where("id = ?", input.ID).Where("user_id = ?", userID).First(&cart).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		if err := db.Delete(&cart).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		utils.HttpRespSuccess(c, http.StatusOK, "Remove from cart", cart)
	})

	// payment gateway
	r.GET("/payment", middleware.Authorization(), func(c *gin.Context) {

		var total int
		var totalPoints int

		ID, _ := c.Get("id")
		userID, ok := ID.(uuid.UUID)
		if !ok {
			utils.HttpRespFailed(c, http.StatusNotFound, "User not found")
			return
		}

		var user model.User
		if err := db.Where("id = ?", userID).First(&user).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		var cart []model.CreditStoreWallet
		if err := db.Where("user_id = ?", userID).Find(&cart).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		for _, v := range cart {
			total += v.Price
			totalPoints += v.Points
		}

		rand.Seed(time.Now().UnixNano())

		midtransClient := coreapi.Client{}
		midtransClient.New(os.Getenv("MIDTRANS_SERVER_KEY"), midtrans.Sandbox)
		orderID := utils.RandomOrderID()
		req := &coreapi.ChargeReq{
			PaymentType: "gopay",
			TransactionDetails: midtrans.TransactionDetails{
				OrderID:  orderID,
				GrossAmt: int64(total),
			},
			Gopay: &coreapi.GopayDetails{
				EnableCallback: true,
				CallbackUrl:    "https://example.com/callback",
			},
			CustomerDetails: &midtrans.CustomerDetails{
				FName: user.Name,
				Email: user.Email,
				Phone: user.Phone,
			},
		}

		resp, err := midtransClient.ChargeTransaction(req)
		if err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		utils.HttpRespSuccess(c, http.StatusOK, "Payment success", resp)

		// update user credit
		user.Point += totalPoints
		if err := db.Save(&user).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		// delete cart
		if err := db.Where("user_id = ?", userID).Delete(&cart).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}

		inputTransactionHistory := model.TransactionHistory{
			UserID:    userID,
			OrderID:   orderID,
			Price:     total,
			Points:    totalPoints,
			CreatedAt: time.Now(),
		}

		if err := db.Create(&inputTransactionHistory).Error; err != nil {
			utils.HttpRespFailed(c, http.StatusNotFound, err.Error())
			return
		}
	})
}