package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/CIA-Labs/love-all-registration/mail"
	"gopkg.in/yaml.v2"
)

type User struct {
	ID        uint      `gorm:"primary_key" json:"id"`
	FirstName string    `gorm:"size:255;not null" json:"first_name"`
	LastName  string    `gorm:"size:255;not null" json:"last_name"`
	Email     string    `gorm:"size:255;unique;not null" json:"email"`
	Password  string    `gorm:"size:255;not null" json:"password"`
	Role      string    `gorm:"size:255;" json:"role"`
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP" json:"updated_at"`
}

// Ref: https://dev.to/ilyakaznacheev/a-clean-way-to-pass-configs-in-a-go-application-1g64
type Config struct {
	GoogleSheetURL struct {
		Endpoint        string `yaml:"endpoint"`
		Role            string `yaml:role`
		Password_suffix string `yaml:password_suffix`
	} `yaml:"google_sheets"`
	Gmail struct {
		EmailSenderName     string `yaml:"EmailSenderName"`
		EmailSenderAddress  string `yaml:"EmailSenderAddress"`
		EmailSenderPassword string `yaml:"EmailSenderPassword"`
	} `yaml:"gmail"`
	Server struct {
		Port     string `yaml:"port"`
		Host     string `yaml:"host"`
		Protocal string `yaml:"protocal"`
		Basepath string `yaml:"basepath"`
		Email    string `yaml:email`
		Password string `yaml:password`
	} `yaml:"server"`
	Database struct {
		Username string `yaml:"user"`
		Password string `yaml:"pass"`
	} `yaml:"database"`
}

func processError(err error) {
	log.Println(err)
	os.Exit(2)
}

func readFile(cfg *Config) {
	f, err := os.Open("config.yml")
	if err != nil {
		processError(err)
	}
	defer f.Close()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(cfg)
	if err != nil {
		processError(err)
	}
}

const (
	minCardNumber = 1000000000000000
	maxCardNumber = 9999999999999999
)

func generateRandomCardNumber() int64 {
	return rand.Int63n(maxCardNumber-minCardNumber+1) + minCardNumber
}

type UserDetails struct {
	CreationTime string `json:Timestamp`
	EmailId      string `json:"Email Address"`
	FirstName    string `json:"First Name"`
	LastName     string `json:"Last Name"`
	PhoneNumber  string `json:"Phone Number"`
	CardType     string `json:"Card Type"`
}
type UserPayload struct {
	Email      string
	First_name string
	Last_name  string
	Password   string
	Role       string
}
type UserResponse struct {
	CreatedAt string `json:"created_at"`
	ID        int    `json:"id"`
	EmailID   string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
}
type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type Google_sheet_data_array [][]string

var Google_sheet_data map[string]Google_sheet_data_array

func PrettyPrint(v interface{}) (err error) {
	b, err := json.MarshalIndent(v, "", "\t")
	if err == nil {
		log.Printf("Response: %s", string(b))
	}
	return err
}

func Login(cfg *Config) (access_token string) {
	type LoginPayload struct {
		Email    string
		Password string
	}
	user_payload := LoginPayload{Email: cfg.Server.Email, Password: cfg.Server.Password}
	userJSON, err := json.Marshal(user_payload)
	if err != nil {
		log.Printf("ERROR: Could not Login to server!")
		return "null"
	}
	user_create_url := cfg.Server.Protocal + "://" + cfg.Server.Host + ":" + cfg.Server.Port + "/login"
	resp, err := http.Post(user_create_url, "application/json", bytes.NewBuffer(userJSON))
	if err != nil {
		log.Printf("ERROR: Could not make POST request to http")
		return "null"
	}
	log.Println("Login:", resp.Status)

	body, err := ioutil.ReadAll(resp.Body)

	var result LoginResponse
	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		log.Printf("ERROR: Unable to parse login response!")
		return "null"
	}
	if result.AccessToken == "" {
		log.Println("Login: Invalid Email OR Password!")
		return "null"
	}
	return (result.AccessToken)
}

func CreateSubscriptions(cfg *Config, token string, SubscriptionType string, EmailID string, ID int) (respone int64, message string) {
	type SubscriptionsPayload struct {
		Card_name string `json:"card_name"`
		Number    int64  `json:"number"`
		User_id   int    `json:"user_id"`
	}
	type CardSubscriptionResponse struct {
		ID        uint
		Card_name string
		Number    int64
		User_id   int
		UserName  string
	}

	rand.Seed(time.Now().UnixNano())

	generatedCards := make(map[int64]bool)
	cardNumber := generateRandomCardNumber()
	if !generatedCards[cardNumber] {
		generatedCards[cardNumber] = true
		log.Printf("Generated Card Number: %d\n", cardNumber)
	}

	// Create a Bearer string by appending string access token
	var bearer = "Bearer " + token
	// add authorization header to the req

	user_payload := SubscriptionsPayload{Card_name: SubscriptionType, Number: cardNumber, User_id: ID}
	PrettyPrint(user_payload)
	userJSON, err := json.Marshal(user_payload)
	if err != nil {
		log.Printf("ERROR: Couldnot parse json!")
		return 0, "null"
	}
	user_create_url := cfg.Server.Protocal + "://" + cfg.Server.Host + ":" + cfg.Server.Port + cfg.Server.Basepath + "/subscriptions"
	req, _ := http.NewRequest("POST", user_create_url, bytes.NewBuffer(userJSON))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Add("Authorization", bearer)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("ERROR: Unable to Generate new card: %d\n", resp.StatusCode)
		processError(err)
	}
	log.Println("CreateCard:", resp.Status)

	body, err := ioutil.ReadAll(resp.Body)

	var result CardSubscriptionResponse
	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		log.Printf("ERROR: Unable to parse login response!")
		return 0, "null"
	}
	return result.Number, "nil"
}

func searchUserByEmailID(cfg *Config, token string, user_email string) (isExists bool) {
	type Users struct {
		Created_at string `json:"created_at"`
		ID         int    `json:"id"`
		Email      string `json:"email"`
		First_name string `json:"first_name"`
		Last_name  string `json:"last_name"`
		Role       string `json:"role"`
	}
	type GetAllUsersResponse struct {
		Data []Users
	}
	// Create a Bearer string by appending string access token
	var bearer = "Bearer " + token

	user_create_url := cfg.Server.Protocal + "://" + cfg.Server.Host + ":" + cfg.Server.Port + cfg.Server.Basepath + "/users"

	req, _ := http.NewRequest("GET", user_create_url, nil)

	// add authorization header to the req
	req.Header.Add("Authorization", bearer)

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("ERROR: Unable to fetch Registered users from server: %d\n", res.StatusCode)
		processError(err)
	}

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)
	log.Println("Get users:", res.Status)
	var result GetAllUsersResponse
	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		log.Printf("ERROR: Unable to parse login response!")
		return true
	}
	for _, user := range result.Data {
		if user.Email == user_email {
			return true
		}
	}
	return false
}

func createUser(cfg *Config, user UserDetails) (status int) {
	// Password format : FirstName@loveall
	static_password_per_user := user.FirstName + cfg.GoogleSheetURL.Password_suffix
	user_payload := UserPayload{Email: user.EmailId, First_name: user.FirstName, Last_name: user.LastName, Password: static_password_per_user, Role: cfg.GoogleSheetURL.Role}
	userJSON, err := json.Marshal(user_payload)
	if err != nil {
		log.Printf("ERROR: Could not parse user details")
	}
	log.Println("Creating: ", user)
	err = PrettyPrint(user)
	if err != nil {
		processError(err)
	}

	user_create_url := cfg.Server.Protocal + "://" + cfg.Server.Host + ":" + cfg.Server.Port + cfg.Server.Basepath + "/users"
	resp, err := http.Post(user_create_url, "application/json", bytes.NewBuffer(userJSON))
	if err != nil {
		log.Printf("ERROR: Could not make POST request to http")
		return -1
	}
	log.Printf(resp.Status)

	body, err := ioutil.ReadAll(resp.Body)

	var result UserResponse
	err = json.Unmarshal([]byte(body), &result)
	if err != nil {
		log.Printf("Error unmarshaling data from request.")
		return -1
	}
	return result.ID
}

func main() {
	var cfg Config
	readFile(&cfg)

	// If the file doesn't exist, create it or append to the file
	file, err := os.OpenFile("love-all-users-onboarding.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		processError(err)
	}

	log.SetOutput(file)
	log.Println("Starting!")

	// Read the returned response
	gogle_sheet_url := string(cfg.GoogleSheetURL.Endpoint)
	resp, err := http.Get(gogle_sheet_url)
	if err != nil {
		log.Printf("ERROR: Google Sheet URL respone status code: %d\n", resp.StatusCode)
		processError(err)
	}

	defer resp.Body.Close()

	// Read the body of the response
	Data, err := io.ReadAll(resp.Body)
	if err != nil {
		processError(err)
	}

	// Unmarshall the returned []byte into json
	json.Unmarshal([]byte(Data), &Google_sheet_data)
	err = PrettyPrint(Google_sheet_data["GoogleSheetData"])
	if err != nil {
		processError(err)
	}
	// Ignore header and parse remaining rows [1:]
	var RegisteredUsers []UserDetails
	for _, col := range Google_sheet_data["GoogleSheetData"][1:] {
		user_data_temp := UserDetails{
			string(col[0]),
			string(col[1]),
			string(col[2]),
			string(col[3]),
			string(col[4]),
			string(col[5]),
		}
		RegisteredUsers = append(RegisteredUsers, user_data_temp)
	}

	for _, user := range RegisteredUsers {
		log.Printf("Processing %s membership for user %s %s with EmailID=%s, PhoneNumber=%s \n", user.CardType, user.FirstName, user.LastName, user.EmailId, user.PhoneNumber)
		// Login with admin token
		access_token := Login(&cfg)
		if access_token == "null" {
			log.Println("Login failed!")
			os.Exit(2)
		}
		log.Println("Login success!")
		log.Println("Got token:", access_token)

		// Search if user.EmailId exists
		search_response := searchUserByEmailID(&cfg, access_token, user.EmailId)
		if search_response {
			log.Println(user.EmailId, "already exists!")
		} else {
			log.Println(user.EmailId, " - New user!")
			// Create user , Assign role [user, admin, merchant] using admin
			response := createUser(&cfg, user)
			if response != -1 {
				log.Println("EmailID:", user.EmailId, ",UserID:", response, "created successfully!")
				log.Println("Requesting New card!")
				// Create user , Assign role [user, admin, merchant] using admin
				response, status := CreateSubscriptions(&cfg, access_token, user.CardType, user.EmailId, int(response))
				log.Println(response)
				if status == "nil" {
					log.Println("Card Number:", response, "created successfully!")
					sender := mail.NewGmailSender(cfg.Gmail.EmailSenderName, cfg.Gmail.EmailSenderAddress, cfg.Gmail.EmailSenderPassword)
					subject := "Welcome to LoveAll Beta: Your Exclusive Membership Has Begun!"
					content := fmt.Sprintf("<p>Dear %s %s,</p>"+
						"<p>We are thrilled to welcome you to the LoveAll Beta program and extend our heartfelt gratitude for becoming a part of our exclusive LoveAll membership community. Your decision to join us as a beta user is greatly appreciated, and we are excited to embark on this journey together.</p>"+
						"<p>Card Number:<h2>%d</h2> </p>"+
						"<p>Card Type: %s</p>"+
						"<p>If you have any questions or need assistance at any point, please do not hesitate to contact our support team at <a href=\"mailto:loveall@cialabs.tech\">loveall@cialabs.tech</a></p>"+
						"<p>Let's make LoveAll Beta an incredible experience together!</p><br>"+
						"<p>Warm regards,</p>"+
						"<p>LoveAll Team</p>", user.FirstName, user.LastName, response, user.CardType)
					// contentt := `
					// <h1>LoveAll</h1>
					// <p>Dear %s</p>
					// <p>PFA E-Card of LoveAll membership</p>
					// `
					to := []string{user.EmailId}
					attachFiles := []string{"README.md"}

					err = sender.SendEmail(subject, content, to, nil, nil, attachFiles)
					log.Println(err)
				} else {
					log.Println("ERROR: Card creation failed!")
				}
			} else {
				log.Println("ERROR: EmailID:", user.EmailId, "creation failed!")
			}
		}
	}
}
