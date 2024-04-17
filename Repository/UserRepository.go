package Repository

import (
	"Clarity_go/Interfaces"
	"Clarity_go/Models"
	"context"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"strings"
	"time"
)

type UsersRepository struct {
	mongodb Interfaces.IMongoService
	IToken  Interfaces.IToken
}

func NewUsersRepository(pMongoDb Interfaces.IMongoService, pToken Interfaces.IToken) *UsersRepository {
	return &UsersRepository{
		mongodb: pMongoDb,
		IToken:  pToken,
	}
}

func (ur UsersRepository) RegisterUser(pUserRegisterDto Models.UserRegisterDto) (*mongo.InsertOneResult, *Models.ResponseError) {

	user, err := ur.isUserExists(pUserRegisterDto.Email)
	if err != nil {
		return nil, &Models.ResponseError{
			Message: "Failed to insert user",
			Status:  http.StatusInternalServerError,
		}
	}

	if user {
		return nil, &Models.ResponseError{
			Message: "User already has an account",
			Status:  http.StatusInternalServerError,
		}
	}

	// Connect to the db
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create a new User model from the UserRegisterDto
	newUser := Models.User{
		Id:        primitive.NewObjectID(), // Generate a new ObjectID
		Email:     pUserRegisterDto.Email,  // Assuming you want to use Username as Email
		Password:  pUserRegisterDto.Password,
		CreatedAt: time.Now(),
	}

	// TODO this should probably be in the mongodb service
	// Correctly access the user collection from the mongodb service
	userCollection := ur.mongodb.GetUserCollection()
	result, err := userCollection.InsertOne(ctx, newUser)
	if err != nil {
		return nil, &Models.ResponseError{
			Message: "Failed to insert user",
			Status:  http.StatusInternalServerError,
		}
	}

	return result, nil
}

func (ur UsersRepository) Login(pUserRegisterDto Models.UserRegisterDto) (*Models.User, *Models.ResponseError) {
	userCollection := ur.mongodb.GetUserCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	xUser := Models.User{
		Email:     pUserRegisterDto.Email,
		Password:  pUserRegisterDto.Password,
		CreatedAt: time.Now(),
	}

	err := userCollection.FindOne(ctx, bson.M{"email": xUser.Email}).Decode(&xUser)
	if err != nil {
		return nil,
			&Models.ResponseError{
				Message: "Email or password is incorrect",
				Status:  http.StatusUnauthorized,
			}
	}
	err = bcrypt.CompareHashAndPassword([]byte(xUser.Password), []byte(pUserRegisterDto.Password))
	if err != nil {
		return nil,
			&Models.ResponseError{
				Message: "Email or password is incorrect",
				Status:  http.StatusUnauthorized,
			}
	}

	return &xUser, nil
}

func (ur UsersRepository) GetUserIdByToken(pToken string) (primitive.ObjectID, *Models.ResponseError) {
	xUser, err := ur.IToken.ValidateToken(pToken)
	if err != nil {
		return primitive.NilObjectID, &Models.ResponseError{
			Message: "Failed to validate token",
			Status:  http.StatusInternalServerError,
		}
	}

	return xUser, nil
}

func (ur UsersRepository) GenerateAccessToken(pId primitive.ObjectID) (*string, *Models.ResponseError) {

	token, err := ur.IToken.GenerateAccessToken(pId)
	if err != nil {
		return nil, &Models.ResponseError{
			Message: err.Message,
			Status:  err.Status,
		}
	}
	return token, nil
}

func (ur UsersRepository) GenerateRefreshToken(pId primitive.ObjectID) (*string, *Models.ResponseError) {
	token, err := ur.IToken.GenerateRefreshToken(pId)
	if err != nil {
		return nil, &Models.ResponseError{
			Message: err.Message,
			Status:  err.Status,
		}
	}

	return token, nil
}

func (ur UsersRepository) AddRefreshTokenToDb(userId primitive.ObjectID, refreshToken string) (*mongo.InsertOneResult, *Models.ResponseError) {
	xRefreshTokenCollection := ur.mongodb.GetRefreshTokenCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	xRefreshToken := Models.RefreshToken{
		UserId:    userId,
		Token:     refreshToken,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(time.Minute * 1),
	}

	xResult, err := xRefreshTokenCollection.InsertOne(ctx, xRefreshToken)
	if err != nil {
		return nil, &Models.ResponseError{
			Message: "Failed to update user",
			Status:  http.StatusInternalServerError,
		}
	}

	return xResult, nil
}

func (ur UsersRepository) DeleteRefreshToken(pRefreshToken string) (*mongo.DeleteResult, *Models.ResponseError) {
	xRefreshTokenCollection := ur.mongodb.GetRefreshTokenCollection()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	xResult, err := xRefreshTokenCollection.DeleteOne(ctx, bson.M{"token": pRefreshToken})
	if err != nil {
		return nil, &Models.ResponseError{
			Message: "Failed to delete user",
			Status:  http.StatusInternalServerError,
		}
	}

	// Check if the delete operation actually removed any documents
	if xResult.DeletedCount == 0 {
		return nil, &Models.ResponseError{
			Message: "No refresh token found to delete",
			Status:  http.StatusNotFound, // or http.StatusBadRequest depending on your application logic
		}
	}

	return xResult, nil
}

func (ur UsersRepository) isUserExists(email string) (bool, error) {
	collection := ur.mongodb.GetUserCollection()
	count, err := collection.CountDocuments(context.TODO(), bson.M{"email": strings.ToLower(email)})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
