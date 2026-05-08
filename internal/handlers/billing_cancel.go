package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"quotepadpro/internal/models"

	"github.com/gin-gonic/gin"
)

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

	if strings.TrimSpace(user.StripeSubscriptionID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No active subscription was found for this account."})
		return
	}

	values := url.Values{}
	values.Set("cancel_at_period_end", "true")

	var subscription struct {
		ID                string `json:"id"`
		Status            string `json:"status"`
		CancelAtPeriodEnd bool   `json:"cancel_at_period_end"`
	}

	path := "/v1/subscriptions/" + url.PathEscape(user.StripeSubscriptionID)
	if err := h.postStripeForm(path, values, &subscription); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "failed to cancel subscription",
			"details": err.Error(),
		})
		return
	}

	if subscription.Status != "" {
		user.SubscriptionStatus = subscription.Status
	}

	if err := h.DB.Save(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update subscription status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"cancelled":         true,
		"cancelAtPeriodEnd": subscription.CancelAtPeriodEnd,
		"subscriptionStatus": user.SubscriptionStatus,
	})
}

func (h *BillingHandler) getStripe(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, "https://api.stripe.com"+path, nil)
	if err != nil {
		return err
	}

	req.SetBasicAuth(h.Cfg.StripeSecretKey, "")

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
