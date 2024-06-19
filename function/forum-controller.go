package function

import (
	"database/sql"
	"fmt"
	"net/http"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	
)

const TimeFormat string = "2006-01-02 15:04:05"

type appData struct {
	User           User
	Posts          []Post
	Post           Post
	Tags           []string
	Tag            string
	TotalPosts     int
	Path           string
	WarningMessage string
	SessionOpen    bool
}

type appError struct {
	Code    int
	Message string
}

type AppHandler func(http.ResponseWriter, *http.Request) *appError

func (fn AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
			w.WriteHeader(500)
			Render(w, "error-page.html", appError{500, http.StatusText(500)})
		}
	}()
	if appErr := fn(w, r); appErr != nil {
		w.WriteHeader(appErr.Code)
		appErr.Message = http.StatusText(appErr.Code)
		Render(w, "error-page.html", appErr)
	}
}

func Home(w http.ResponseWriter, r *http.Request) *appError {
	if r.URL.Path != "/" {
		return &appError{Code: http.StatusNotFound}
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	posts, err := GetAllPosts()
	if err != nil {
		return &appError{Code: http.StatusInternalServerError}
	}
	for i := 0; i < len(*posts); i++ {
		(*posts)[i].When = When((*posts)[i].CreatedAt)
	}

	tags, err := GetAllTags()
	if err != nil {
		return &appError{Code: http.StatusInternalServerError}
	}
	username, isSessionOpen := ValidSession(r)
	data := &appData{
		SessionOpen: isSessionOpen,
		User:        User{Username: username},
		Posts:       *posts,
		Tags:        tags,
		TotalPosts:  len(*posts),
	}
	Render(w, "home-page.html", data)
	return nil
}

func Profile(w http.ResponseWriter, r *http.Request) *appError {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	data := &appData{}
	var username string
	login := r.URL.Query().Get("username")
	username, data.SessionOpen = ValidSession(r)
	if login != "" && login != username {
		username = login
	}
	user, err := GetUser(username)
	if err != nil {
		return &appError{Code: http.StatusNotFound}
	}
	posts, err := GetPostsCreatedByUser(user.ID)
	if err != nil {
		return &appError{Code: 500}
	}
	for _, post := range *posts {
		user.Reputation += post.Votes.Likes - post.Votes.Dislikes
	}
	data.User = *user
	data.TotalPosts = len(*posts)
	Render(w, "profile-page.html", data)
	return nil
}

func Signin(w http.ResponseWriter, r *http.Request) *appError {
	nextURL := r.FormValue("next")
	if nextURL == "" {
		nextURL = "/"
	}
	vote := r.FormValue("vote")
	if vote != "" && nextURL != "/" {
		nextURL += "&vote=" + vote
	}
	_, isSessionOpen := ValidSession(r)
	if isSessionOpen {
		http.Redirect(w, r, nextURL, http.StatusSeeOther)
		return nil
	}
	data := &appData{Path: nextURL}
	switch r.Method {
	case http.MethodGet:
		Render(w, "signin-page.html", data)
	case http.MethodPost:
		username := r.FormValue("username")
		password := r.FormValue("password")
		if username == "" || password == "" {
			return &appError{Code: http.StatusBadRequest}
		}
		user, err := GetUser(username)
		if err != nil || !DoPasswordsMatch(user.Password, password) {
			data.WarningMessage = "Nom d'utilisateur ou mot de passe incorrect."
			Render(w, "signin-page.html", data)
			return nil
		}
		NewSessionToken(w, username)
		http.Redirect(w, r, nextURL, http.StatusSeeOther)
	default:
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	return nil
}

func Signout(w http.ResponseWriter, r *http.Request) *appError {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	_, isSessionOpen := ValidSession(r)
	if !isSessionOpen {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	c, _ := r.Cookie("session_token")
	sessions.Delete(c.Value)
	http.SetCookie(w, &http.Cookie{
		Name:    "session_token",
		Value:   "",
		Expires: time.Now(),
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func Signup(w http.ResponseWriter, r *http.Request) *appError {
	_, isSessionOpen := ValidSession(r)
	if isSessionOpen {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	data := &appData{}
	switch r.Method {
	case http.MethodGet:
		Render(w, "signup-page.html", data)
	case http.MethodPost:
		time := time.Now().Format(TimeFormat)
		user := User{
			Username:  r.FormValue("username"),
			Email:     r.FormValue("email"),
			Password:  r.FormValue("password"),
			CreatedAt: time,
		}
		confirmPwd := r.FormValue("confirm")
		//----------
		if user.Username == "" || user.Email == "" || user.Password == "" || confirmPwd == "" {
			return &appError{Code: http.StatusBadRequest}
		}
		if len(user.Username) > 16 || len(user.Password) < 6 || user.Password != confirmPwd {
			return &appError{Code: 400}
		}
		loginExpr := "^[a-zA-Z0-9]*$"
		emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,4}$`)
		if rex, _ := regexp.Compile(loginExpr); !rex.MatchString(user.Username) {
			return &appError{Code: 400}
		}
		if _, err := mail.ParseAddress(user.Email); err != nil || !emailRegex.MatchString(user.Email) {
			return &appError{Code: 400}
		}
		//----------
		hashedPwd, err := HashPassword(user.Password)
		if err != nil {
			return &appError{Code: 500}
		}
		user.Password = hashedPwd
		if err = CreateUser(user); err != nil {
			data.WarningMessage = "Le nom d'utilisateur ou l'adresse électronique existe déjà" //be more clear
			Render(w, "signup-page.html", data)
			return nil
		}
		NewSessionToken(w, user.Username)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		return &appError{Code: 405}
	}
	return nil
}

func CreatePost2(w http.ResponseWriter, r *http.Request) *appError {
	username, isSessionOpen := ValidSession(r)
	if !isSessionOpen {
		http.Redirect(w, r, "/accounts/login?next=/create/post", http.StatusFound)
		return nil
	}
	data := &appData{SessionOpen: isSessionOpen}
	switch r.Method {
	case http.MethodGet:
		Render(w, "create-page.html", data)
	case http.MethodPost:
		time := time.Now().Format(TimeFormat)
		post := &Post{
			Title:     r.FormValue("title"),
			Text:      r.FormValue("text"),
			Tags:      strings.Split(r.FormValue("tags"), " "),
			CreatedAt: time,
		}
		//------------
		if strings.TrimSpace(post.Title) == "" || strings.TrimSpace(post.Text) == "" {
			return &appError{Code: http.StatusBadRequest}
		}
		if utf8.RuneCountInString(post.Title) > 100 || utf8.RuneCountInString(post.Text) > 10000 {
			return &appError{Code: http.StatusBadRequest}
		}
		if len(post.Tags) > 50 {
			return &appError{Code: http.StatusBadRequest}
		}
		for _, tag := range post.Tags {
			if strings.Contains(tag, " ") || utf8.RuneCountInString(tag) > 30 || tag == "" {
				return &appError{Code: http.StatusBadRequest}
			}
		}
		//------------
		user, err := GetUser(username)
		if err != nil {
			return &appError{Code: 500}
		}
		post.UserID = user.ID
		post.Username = username
		if err = CreatePost(post); err != nil {
			return &appError{Code: 500}
		}
		if err = CreateTags(post.ID, post.Tags); err != nil {
			return &appError{Code: 500}
		}
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	return nil
}

func CreateComment2(w http.ResponseWriter, r *http.Request) *appError {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	username, isSessionOpen := ValidSession(r)
	if !isSessionOpen {
		http.Redirect(w, r, "/accounts/login?next=/create/comment", http.StatusFound)
		return nil
	}
	postID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return &appError{Code: http.StatusNotFound}
	}
	user, err := GetUser(username)
	if err != nil {
		return &appError{Code: http.StatusInternalServerError}
	}
	time := time.Now().Format(TimeFormat)
	comment := &Comment{
		PostID:    int64(postID),
		UserID:    user.ID,
		Username:  username,
		Text:      r.FormValue("text"),
		CreatedAt: time,
	}
	//-------------
	if strings.TrimSpace(comment.Text) == "" || utf8.RuneCountInString(comment.Text) > 200 {
		return &appError{Code: http.StatusBadRequest}
	}
	//-------------
	if err = CreateComment(*comment); err != nil {
		return &appError{Code: 500}
	}
	next := fmt.Sprintf("/posts?id=%v", postID)
	http.Redirect(w, r, next, http.StatusSeeOther)
	return nil
}

func VotePost2(w http.ResponseWriter, r *http.Request) *appError {
	username, isSessionOpen := ValidSession(r)
	postID, err := strconv.Atoi(r.URL.Query().Get("id"))
	if err != nil {
		return &appError{Code: http.StatusBadRequest}
	}
	vote, err := strconv.Atoi(r.URL.Query().Get("vote"))
	if err != nil {
		return &appError{Code: http.StatusBadRequest}
	}
	if vote != 1 && vote != -1 {
		return &appError{Code: http.StatusBadRequest}
	}
	nextPath := fmt.Sprintf("/accounts/login?next=/vote/post?id=%v&vote=%v", postID, vote)
	if !isSessionOpen {
		http.Redirect(w, r, nextPath, http.StatusFound)
		return nil
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	user, err := GetUser(username)
	if err != nil {
		return &appError{Code: http.StatusBadRequest}
	}
	if _, err = GetPostById(int64(postID)); err != nil {
		return &appError{Code: http.StatusBadRequest}
	}

	if err = VotePost(user.ID, int64(postID), vote); err != nil {
		return &appError{Code: 500}
	}
	next := fmt.Sprintf("/posts?id=%v", postID)
	http.Redirect(w, r, next, http.StatusSeeOther)
	return nil
}

func VoteComment2(w http.ResponseWriter, r *http.Request) *appError {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	username, isSessionOpen := ValidSession(r)
	cmtID, err := strconv.ParseInt(r.URL.Query().Get("id"), 0, 0)
	if err != nil {
		return &appError{Code: http.StatusBadRequest}
	}
	vote, err := strconv.Atoi(r.URL.Query().Get("vote"))
	if err != nil {
		return &appError{Code: http.StatusBadRequest}
	}
	if vote != 1 && vote != -1 {
		return &appError{Code: http.StatusBadRequest}
	}
	nextPath := fmt.Sprintf("/accounts/login?next=/vote/comment?id=%v&vote=%v", cmtID, vote)
	if !isSessionOpen {
		http.Redirect(w, r, nextPath, http.StatusFound)
		return nil
	}
	user, err := GetUser(username)
	if err != nil {
		return &appError{Code: http.StatusBadRequest}
	}
	comment, err := GetCommentByID(cmtID)
	if err == sql.ErrNoRows {
		return &appError{Code: http.StatusBadRequest}
	} else if err != nil {
		return &appError{Code: 500}
	}
	if err = VoteComment(user.ID, cmtID, vote); err != nil {
		return &appError{Code: 500}
	}
	next := fmt.Sprintf("/posts?id=%v", comment.PostID)
	http.Redirect(w, r, next, http.StatusSeeOther)
	return nil
}

func GetPosts(w http.ResponseWriter, r *http.Request) *appError { //FILTER FUNC
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	_, isSessionOpen := ValidSession(r)
	var data = &appData{SessionOpen: isSessionOpen}
	tag := r.FormValue("tag")
	data.Tag = tag
	tag = strings.TrimPrefix(tag, "#")
	if tag != "" {
		posts, err := GetPostsByTag(tag)
		if err == sql.ErrNoRows {
			Render(w, "home-page.html", data)
			return nil
		} else if err != nil {
			return &appError{Code: 500}
		}
		data.Posts = *posts
		data.TotalPosts = len(*posts)
	} else {
		posts, err := GetAllPosts()
		if err != nil {
			return &appError{Code: 500}
		}
		data.Posts = *posts
		data.TotalPosts = len(*posts)
	}
	allTags, err := GetAllTags()
	if err != nil {
		return &appError{Code: 500}
	}
	data.Tags = allTags
	Render(w, "home-page.html", data)
	return nil
}

func GetPostByID(w http.ResponseWriter, r *http.Request) *appError {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	val := r.URL.Query().Get("id")
	if val == "" {
		return GetPosts(w, r)
	}
	postID, err := strconv.ParseInt(val, 0, 0)
	if err != nil {
		return &appError{Code: http.StatusNotFound}
	}
	post, err := GetPostById(postID)
	if err != nil {
		return &appError{Code: 404}
	}
	post.When = When(post.CreatedAt)
	for i := 0; i < len(post.Comments); i++ {
		post.Comments[i].When = When(post.Comments[i].CreatedAt)
	}

	username, isSessionOpen := ValidSession(r)
	data := &appData{
		SessionOpen: isSessionOpen,
		User:        User{Username: username},
		Post:        *post,
	}
	Render(w, "post-page.html", data)
	return nil
}

func GetPostsCreated(w http.ResponseWriter, r *http.Request) *appError {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	login := r.URL.Query().Get("username")
	username, isSessionOpen := ValidSession(r)
	if login == "" {
		login = username
	}
	user, err := GetUser(login)
	if err != nil {
		return &appError{Code: http.StatusNotFound}
	}
	posts, err := GetPostsCreatedByUser(user.ID)
	if err != nil {
		return &appError{Code: http.StatusInternalServerError}
	}
	for i := 0; i < len(*posts); i++ {
		(*posts)[i].When = When((*posts)[i].CreatedAt)
	}
	tags, err := GetAllTags()
	if err != nil {
		return &appError{Code: 500}
	}
	Render(w, "home-page.html", appData{
		SessionOpen: isSessionOpen,
		User:        *user,
		Posts:       *posts,
		Tags:        tags,
		TotalPosts:  len(*posts),
	})
	return nil
}

func GetPostsLiked(w http.ResponseWriter, r *http.Request) *appError {
	username, isSessionOpen := ValidSession(r)
	if !isSessionOpen {
		http.Redirect(w, r, "/accounts/login/?next=/posts/mylikes", http.StatusFound)
		return nil
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		return &appError{Code: http.StatusMethodNotAllowed}
	}
	user, _ := GetUser(username)
	posts, err := GetPostsVotedByUser(user.ID, 1)
	if err != nil {
		return &appError{Code: 500}
	}
	for i := 0; i < len(*posts); i++ {
		(*posts)[i].When = When((*posts)[i].CreatedAt)
	}
	tags, err := GetAllTags()
	if err != nil {
		return &appError{Code: 500}
	}
	data := &appData{
		SessionOpen: isSessionOpen,
		Posts:       *posts,
		User:        *user,
		Tags:        tags,
		TotalPosts:  len(*posts),
	}
	Render(w, "home-page.html", data)
	return nil
}
