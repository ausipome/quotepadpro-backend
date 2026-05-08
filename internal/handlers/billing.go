// quotepadpro-backend/internal/handlers/billing.go

package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"quotepadpro/internal/config"
	"quotepadpro/internal/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BillingHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

func NewBillingHandler(db *gorm.DB, cfg *config.Config) *BillingHandler {
	return &BillingHandler{DB: db, Cfg: cfg}
}

type createCheckoutSessionResponse struct {
	ClientSecret string `json:"clientSecret"`
}

type stripeCustomerResponse struct {
	ID string `json:"id"`
}

type stripeSessionResponse struct {
	ID           string `json:"id"`
	ClientSecret string `json:"client_secret"`
}

type stripeSubscriptionResponse struct {
	ID                string `json:"id"`
	Status            string `json:"status"`
	CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
}

func (h *BillingHandler) CreateCheckoutSession(c *gin.Context) {
	userID := c.MustGet("userId").(uint)

	if h.Cfg.StripeSecretKey == "" || h.Cfg.StripePriceID == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Stripe is not configured"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if user.StripeCustomerID == "" {
		values := url.Values{}
		values.Set("email", user.Email)
		values.Set("name", user.Name)
		values.Set("metadata[userId]", strconv.FormatUint(uint64(user.ID), 10))

		var customer stripeCustomerResponse
		if err := h.postStripeForm("/v1/customers", values, &customer); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to create Stripe customer",
				"details": err.Error(),
			})
			return
		}

		user.StripeCustomerID = customer.ID
		if user.SubscriptionStatus == "" {
			user.SubscriptionStatus = "incomplete"
		}

		if err := h.DB.Save(&user).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save Stripe customer"})
			return
		}
	}

	frontendURL := strings.TrimRight(h.Cfg.FrontendURL, "/")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	values := url.Values{}
	values.Set("mode", "subscription")
	values.Set("ui_mode", "embedded")
	values.Set("customer", user.StripeCustomerID)
	values.Set("line_items[0][price]", h.Cfg.StripePriceID)
	values.Set("line_items[0][quantity]", "1")
	values.Set("subscription_data[trial_period_days]", "30")
	values.Set("subscription_data[metadata][userId]", strconv.FormatUint(uint64(user.ID), 10))
	values.Set("metadata[userId]", strconv.FormatUint(uint64(user.ID), 10))
	values.Set("return_url", frontendURL+"/checkout/complete?session_id={CHECKOUT_SESSION_ID}")

	var session stripeSessionResponse
	if err := h.postStripeForm("/v1/checkout/sessions", values, &session); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to create checkout session",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, createCheckoutSessionResponse{ClientSecret: session.ClientSecret})
}

func (h *BillingHandler) CancelSubscription(c *gin.Context) {
	userID := c.MustGet("userId").(uint)

	if h.Cfg.StripeSecretKey == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Stripe is not configured"})
		return
	}

	var user models.User
	if err := h.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	if user.StripeSubscriptionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active Stripe subscription was found for this account"})
		return
	}

	values := url.Values{}
	values.Set("cancel_at_period_end", "true")

	var subscription stripeSubscriptionResponse
	if err := h.postStripeForm("/v1/subscriptions/"+url.PathEscape(user.StripeSubscriptionID), values, &subscription); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to cancel subscription",
			"details": err.Error(),
		})
		return
	}

	updates := map[string]any{
		"cancel_at_period_end": true,
	}

	if subscription.Status != "" {
		updates["subscription_status"] = subscription.Status
	}

	if subscription.ID != "" {
		updates["stripe_subscription_id"] = subscription.ID
	}

	if err := h.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update subscription status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":             "Subscription will cancel at the end of the current period",
		"subscriptionStatus":  subscription.Status,
		"cancelAtPeriodEnd":   true,
		"stripeSubscriptionId": user.StripeSubscriptionID,
	})
}

func (h *BillingHandler) StripeWebhook(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}

	if h.Cfg.StripeWebhookSecret != "" && !verifyStripeSignature(body, c.GetHeader("Stripe-Signature"), h.Cfg.StripeWebhookSecret) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid signature"})
		return
	}

	var event struct {
		Type string `json:"type"`
		Data struct {
			Object map[string]any `json:"object"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid event"})
		return
	}

	switch event.Type {
	case "checkout.session.completed":
		h.handleCheckoutCompleted(event.Data.Object)

	case "customer.subscription.created",
		"customer.subscription.updated",
		"customer.subscription.deleted":
		h.handleSubscriptionChanged(event.Data.Object)

	case "invoice.paid":
		h.handleInvoicePaid(event.Data.Object)

	case "invoice.payment_failed":
		h.handleInvoicePaymentFailed(event.Data.Object)

	case "invoice.payment_action_required":
		h.handleInvoicePaymentActionRequired(event.Data.Object)

	case "invoice.finalization_failed":
		h.handleInvoiceFinalizationFailed(event.Data.Object)
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

func (h *BillingHandler) handleCheckoutCompleted(obj map[string]any) {
	userID := metadataUserID(obj)
	customerID := stringField(obj, "customer")
	subscriptionID := stringField(obj, "subscription")

	if userID == 0 && customerID == "" {
		return
	}

	updates := map[string]any{
		"stripe_customer_id":     customerID,
		"stripe_subscription_id": subscriptionID,
		"subscription_status":    "trialing",
		"cancel_at_period_end":   false,
	}

	if userID != 0 {
		h.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
		return
	}

	h.DB.Model(&models.User{}).Where("stripe_customer_id = ?", customerID).Updates(updates)
}

func (h *BillingHandler) handleSubscriptionChanged(obj map[string]any) {
	customerID := stringField(obj, "customer")
	subscriptionID := stringField(obj, "id")
	status := stringField(obj, "status")

	if customerID == "" || status == "" {
		return
	}

	updates := map[string]any{
		"stripe_subscription_id": subscriptionID,
		"subscription_status":    status,
		"cancel_at_period_end":   boolField(obj, "cancel_at_period_end"),
	}

	h.DB.Model(&models.User{}).Where("stripe_customer_id = ?", customerID).Updates(updates)
}

func (h *BillingHandler) handleInvoicePaid(obj map[string]any) {
	customerID := stringField(obj, "customer")
	subscriptionID := stringField(obj, "subscription")

	if customerID == "" {
		return
	}

	updates := map[string]any{
		"stripe_subscription_id": subscriptionID,
		"subscription_status":    "active",
	}

	h.DB.Model(&models.User{}).Where("stripe_customer_id = ?", customerID).Updates(updates)
}

func (h *BillingHandler) handleInvoicePaymentFailed(obj map[string]any) {
	customerID := stringField(obj, "customer")
	subscriptionID := stringField(obj, "subscription")

	if customerID == "" {
		return
	}

	updates := map[string]any{
		"stripe_subscription_id": subscriptionID,
		"subscription_status":    "past_due",
	}

	h.DB.Model(&models.User{}).Where("stripe_customer_id = ?", customerID).Updates(updates)
}

func (h *BillingHandler) handleInvoicePaymentActionRequired(obj map[string]any) {
	customerID := stringField(obj, "customer")
	subscriptionID := stringField(obj, "subscription")

	if customerID == "" {
		return
	}

	updates := map[string]any{
		"stripe_subscription_id": subscriptionID,
		"subscription_status":    "incomplete",
	}

	h.DB.Model(&models.User{}).Where("stripe_customer_id = ?", customerID).Updates(updates)
}

func (h *BillingHandler) handleInvoiceFinalizationFailed(obj map[string]any) {
	customerID := stringField(obj, "customer")
	subscriptionID := stringField(obj, "subscription")

	if customerID == "" {
		return
	}

	updates := map[string]any{
		"stripe_subscription_id": subscriptionID,
		"subscription_status":    "incomplete",
	}

	h.DB.Model(&models.User{}).Where("stripe_customer_id = ?", customerID).Updates(updates)
}

func (h *BillingHandler) postStripeForm(path string, values url.Values, out any) error {
	req, err := http.NewRequest(http.MethodPost, "https://api.stripe.com"+path, bytes.NewBufferString(values.Encode()))
	if err != nil {
		return err
	}

	req.SetBasicAuth(h.Cfg.StripeSecretKey, "")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("stripe error: %s", string(body))
	}

	return json.Unmarshal(body, out)
}

func verifyStripeSignature(payload []byte, header string, secret string) bool {
	parts := strings.Split(header, ",")
	var timestamp string
	signatures := []string{}

	for _, part := range parts {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			continue
		}

		switch kv[0] {
		case "t":
			timestamp = kv[1]
		case "v1":
			signatures = append(signatures, kv[1])
		}
	}

	if timestamp == "" || len(signatures) == 0 {
		return false
	}

	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil || time.Since(time.Unix(ts, 0)) > 5*time.Minute {
		return false
	}

	signedPayload := []byte(timestamp + "." + string(payload))
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(signedPayload)
	expected := hex.EncodeToString(mac.Sum(nil))

	for _, sig := range signatures {
		if hmac.Equal([]byte(expected), []byte(sig)) {
			return true
		}
	}

	return false
}

func metadataUserID(obj map[string]any) uint {
	metadata, ok := obj["metadata"].(map[string]any)
	if !ok {
		return 0
	}

	raw, ok := metadata["userId"].(string)
	if !ok || raw == "" {
		return 0
	}

	parsed, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}

	return uint(parsed)
}

func stringField(obj map[string]any, key string) string {
	val, ok := obj[key].(string)
	if !ok {
		return ""
	}

	return val
}

func boolField(obj map[string]any, key string) bool {
	val, ok := obj[key].(bool)
	return ok && val
}
