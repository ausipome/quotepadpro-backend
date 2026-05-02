package models

import "time"

type User struct {
	ID                     uint       `gorm:"primaryKey" json:"id"`
	Name                   string     `json:"name"`
	Email                  string     `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash           string     `json:"-"`
	EmailVerified          bool       `gorm:"default:false" json:"emailVerified"`
	EmailVerifyToken       string     `json:"-"`
	EmailVerifyExpiresAt   *time.Time `json:"-"`
	ResetPasswordToken     string     `json:"-"`
	ResetPasswordExpiresAt *time.Time `json:"-"`
	BusinessName           string     `json:"businessName"`
	Phone                  string     `json:"phone"`
	LogoURL                string     `json:"logoUrl"`
	LogoFilename           string     `json:"logoFilename"`
	StripeCustomerID       string     `gorm:"index" json:"stripeCustomerId"`
	StripeSubscriptionID   string     `gorm:"index" json:"stripeSubscriptionId"`
	SubscriptionStatus     string     `gorm:"default:incomplete" json:"subscriptionStatus"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
	Contacts               []Contact  `json:"contacts,omitempty"`
	Quotes                 []Quote    `json:"quotes,omitempty"`
}

type Contact struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `gorm:"index;not null" json:"userId"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone"`
	Company   string    `json:"company"`
	Address1  string    `json:"address1"`
	Address2  string    `json:"address2"`
	City      string    `json:"city"`
	County    string    `json:"county"`
	Postcode  string    `json:"postcode"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Quote struct {
	ID            uint        `gorm:"primaryKey" json:"id"`
	UserID        uint        `gorm:"index;not null" json:"userId"`
	ContactID     *uint       `gorm:"index" json:"contactId"`
	QuoteNumber   string      `gorm:"index" json:"quoteNumber"`
	Title         string      `json:"title"`
	Status        string      `gorm:"default:draft" json:"status"`
	QuoteDate     *time.Time  `json:"quoteDate"`
	ExpiryDate    *time.Time  `json:"expiryDate"`
	Notes         string      `json:"notes"`
	Subtotal      float64     `json:"subtotal"`
	DiscountType  string      `json:"discountType"`
	DiscountValue float64     `json:"discountValue"`
	VATMode       string      `json:"vatMode"`
	VATRate       float64     `json:"vatRate"`
	VATAmount     float64     `json:"vatAmount"`
	Total         float64     `json:"total"`
	PublicID      string      `gorm:"uniqueIndex" json:"publicId"`
	AcceptedAt    *time.Time  `json:"acceptedAt"`
	CreatedAt     time.Time   `json:"createdAt"`
	UpdatedAt     time.Time   `json:"updatedAt"`
	Items         []QuoteItem `json:"items,omitempty"`
	Contact       *Contact    `json:"contact,omitempty"`
}

type QuoteItem struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	QuoteID     uint      `gorm:"index;not null" json:"quoteId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Quantity    float64   `json:"quantity"`
	UnitPrice   float64   `json:"unitPrice"`
	LineTotal   float64   `json:"lineTotal"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}
