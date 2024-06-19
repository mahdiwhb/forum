package function

import (
	"net/http"
	
)

func RegisterForumRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/css/", http.StripPrefix("/css", http.FileServer(http.Dir("./ui/css"))))
	mux.Handle("/img/", http.StripPrefix("/img", http.FileServer(http.Dir("./ui/img"))))
	mux.Handle("/js/", http.StripPrefix("/js", http.FileServer(http.Dir("./ui/js"))))

	mux.Handle("/", AppHandler(Home))
	mux.Handle("/profile", AppHandler(Profile))
	mux.Handle("/accounts/login", AppHandler(Signin))
	mux.Handle("/accounts/logout", AppHandler(Signout))
	mux.Handle("/accounts/register", AppHandler(Signup))

	mux.Handle("/create/post", AppHandler(CreatePost2))
	mux.Handle("/create/comment", AppHandler(CreateComment2))
	mux.Handle("/vote/post", AppHandler(VotePost2))
	mux.Handle("/vote/comment", AppHandler(VoteComment2))

	mux.Handle("/posts/filter", AppHandler(GetPosts))
	mux.Handle("/posts", AppHandler(GetPostByID))
	mux.Handle("/posts/myposts", AppHandler(GetPostsCreated))
	mux.Handle("/posts/mylikes", AppHandler(GetPostsLiked))

	return mux
}
