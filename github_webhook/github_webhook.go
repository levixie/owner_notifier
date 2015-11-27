package main

import "net/http"
import "io/ioutil"
import "flag"
import "crypto/hmac"
import "github.com/golang/glog"
import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/smtp"
	"os"
	"strings"
	"time"
)

type SETTINGS struct {
	Host         string `json:"host"`
	Token        string `json:"token"`
	MailHost     string `json:"mailhost"`
	MailPort     string `json:"mailport"`
	MailSMTPAuth bool   `json:"mailsmptauth"`
	MailUser     string `json:"mailuser"`
	MailPassword string `json:"mailpassword"`
	OwnersFile   string `json:"ownersfile"`
	Branch       string `json:"branch"`
}

type GitHubCommitter struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	UserName string `json:username"`
}

type GitHubCommit struct {
	ID        string          `json:"id"`
	Message   string          `json:"message"`
	Url       string          `json:"url"`
	Committer GitHubCommitter `json:"committer"`
	Modified  []string        `json:"modified"`
	Added     []string        `json:"added"`
	Removed   []string        `json:"removed"`
	Distinct  bool            `json:"distinct"`
}

type GitRepo struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Fullname string `json:"full_name"`
}

type Sender struct {
	Login string `json:"login"`
}

type GitHubPushEvent struct {
	Ref        string         `json:"ref"`
	Compare    string         `json:"compare"`
	Commits    []GitHubCommit `json:"commits"`
	Repository GitRepo        `json:"repository"`
	Sender     Sender         `json:"sender"`
}

func unencryptedSendMail(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
	c, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()

	if a != nil {
		if err = c.Auth(a); err != nil {
			return err
		}
	}
	if err = c.Mail(from); err != nil {
		return err
	}
	for _, addr := range to {
		if err = c.Rcpt(addr); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	_, err = w.Write(msg)
	if err != nil {
		return err
	}
	err = w.Close()
	if err != nil {
		return err
	}
	return c.Quit()
}

var cfg SETTINGS
var ownerFile = make(map[string]string)

func SendEmail(
	commit GitHubCommit,
	ownedFiles []string,
	owners []string,
	realOwner string,
	Repository string) {

	// Set up authentication information.
	auth := smtp.PlainAuth(
		"",
		cfg.MailUser,
		cfg.MailPassword,
		cfg.MailHost,
	)

	modifier := commit.Committer.Email[:strings.Index(commit.Committer.Email, "@")-1]
	title := modifier + " commit: " + commit.Message[:125]

	ownedFilesLi := ""
	for _, file := range ownedFiles {
		ownedFilesLi += "<li>" + file + "</li>"
	}

	modifiedLi := ""
	for _, file := range commit.Modified {
		modifiedLi += "<li>" + file + "</li>"
	}

	removedLi := ""
	for _, file := range commit.Removed {
		removedLi += "<li>" + file + "</li>"
	}
	body := fmt.Sprintf(
		`<html>
		Committer: %s<br><br>
		Email: %s<br><br>
		Branch: %s<br><br>
		Commit Message: %s<br><br>
        Files were <a href=%s>changed</a><br><br>

		Files You Owned:
        <ul style="list-style-type:disc">
		%s
        </ul>
        <br>
        All Modified:
        <ul style="list-style-type:disc">
        %s
        </ul>
        <br>
        All Removed:
        <ul style="list-style-type:disc">
        %s
        </ul>
        <br>
        </html>
        `,
		commit.Committer.Name,
		commit.Committer.Email,
		Repository,
		commit.Message,
		"https://cr.houzz.net/rALL"+commit.ID,
		ownedFilesLi,
		modifiedLi,
		removedLi,
	)

	header := make(map[string]string)
	header["From"] = "code+changed+alert@cr.houzz.net"
	header["To"] = strings.Join(owners, ",")
	header["Subject"] = title
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + base64.StdEncoding.EncodeToString([]byte(body))

	err := unencryptedSendMail(
		cfg.MailHost+":"+cfg.MailPort,
		auth,
		"code+changed+alert@cr.houzz.net",
		owners,
		[]byte(message),
		//[]byte("This is the email body."),
	)

	if err != nil {
		glog.Errorln(err)
	}
}

func examCommit(body []byte) {
	var prettyJSON bytes.Buffer
	json.Indent(&prettyJSON, body, "", "\t")
	var pushEvent GitHubPushEvent
	err := json.Unmarshal(body, &pushEvent)
	if err != nil {
		glog.V(1).Info("err", err)
	}
	glog.V(2).Info(pushEvent)
	glog.V(3).Info(string(prettyJSON.Bytes()))

	if !strings.Contains(cfg.Branch, pushEvent.Ref) {
		glog.V(1).Infof("don't care about %s", pushEvent.Ref)
		return
	}

	for _, commit := range pushEvent.Commits {
		glog.V(1).Info(
			"New commit: ",
			commit.Distinct,
			" Email:",
			commit.Committer.Email,
			" changed on ",
			pushEvent.Repository.Name,
			" with files ",
			commit.Modified)
		if commit.Distinct {
			changedFiles := make(map[string][]string)
			for _, file := range append(commit.Modified, commit.Removed...) {
				if len(ownerFile[file]) != 0 && !strings.Contains(ownerFile[file], commit.Committer.Email) {
					for _, owner := range strings.Split(ownerFile[file], ";") {
						changedFiles[owner] = append(changedFiles[owner], file)
					}
				}
			}

			for owner, ownedFiles := range changedFiles {
				SendEmail(
					commit,
					ownedFiles,
					[]string{owner},
					owner,
					pushEvent.Ref,
				)

				glog.V(2).Info(
					"Email:",
					commit.Committer.Email,
					" changed on ",
					pushEvent.Repository.Name,
					" with files ",
					commit.Modified,
					commit.Removed,
					" to owner:",
					owner,
					" owning ",
					ownedFiles,
				)
			}
		}
	}
}

// CheckPayloadSignature calculates and verifies SHA1 signature of the given payload
func CheckPayloadSignature(payload []byte, secret string, signature string) (string, bool) {
	if strings.HasPrefix(signature, "sha1=") {
		signature = signature[5:]
	}

	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return expectedMAC, hmac.Equal([]byte(signature), []byte(expectedMAC))
}

func webHookPost(w http.ResponseWriter, req *http.Request) {

	defer req.Body.Close()
	body, _ := ioutil.ReadAll(req.Body)
	_, is_pass := CheckPayloadSignature(body, cfg.Token, req.Header.Get("X-Hub-Signature"))

	//if is_pass && strings.EqualFold(req.Header.Get("X-GitHub-Event"), "push") {
	if is_pass {
		w.Write([]byte("OK"))
		if strings.EqualFold(req.Header.Get("X-GitHub-Event"), "push") {
			go examCommit(body)
		}
	} else {
		http.Error(w, "wrong key", http.StatusBadRequest)
	}
}

func startService(port string) {

	// your http.Handle calls here
	http.Handle("/webHookPost", http.HandlerFunc(webHookPost))

	err := http.ListenAndServe(port, nil)
	if err != nil {
		glog.Fatalf("ListenAndServe: %s", err)
	}
}

func readOwnersFile(fileName string) {

	lmt := time.Now()
	for {
		info, err := os.Stat(fileName)
		if lmt == info.ModTime() {
			glog.V(3).Info(fileName, " doesn't change ", lmt)
			time.Sleep(time.Duration(60) * time.Second)
			continue
		}

		glog.V(3).Info("read owners from ", fileName)
		lmt = info.ModTime()

		file, err := os.Open(fileName)
		if err != nil {
			// err is printable
			// elements passed are separated by space automatically
			fmt.Println("Error:", err)
			return
		}

		reader := csv.NewReader(file)
		var tempOwnerFile = make(map[string]string)
		for {
			// read just one record, but we could ReadAll() as well
			record, err := reader.Read()
			// end-of-file is fitted into err
			if err == io.EOF {
				break
			} else if err != nil {
				glog.V(0).Info("Error:", err)
			}

			if len(record) > 2 {
				repoFile := record[0]
				owner := record[2]
				tempOwnerFile[repoFile] = owner
			}
		}

		ownerFile = tempOwnerFile
		file.Close()

	}
}

func setOwnersByFile(fileName string, gitRoot string) {
	glog.V(3).Info("read owners from ", fileName)
	file, err := os.Open(fileName)
	if err != nil {
		// err is printable
		// elements passed are separated by space automatically
		fmt.Println("Error:", err)
		return
	}
	defer file.Close()

	reader := csv.NewReader(file)
	for {
		// read just one record, but we could ReadAll() as well
		record, err := reader.Read()
		// end-of-file is fitted into err
		if err == io.EOF {
			break
		} else if err != nil {
			glog.V(0).Info("Error:", err)
		}

		if len(record) > 2 && len(record[2]) > 0 {
			repoFile := record[0]
			if strings.HasSuffix(repoFile, ".py") {
				continue
			}

			owner := record[2]
			body, err := ioutil.ReadFile(gitRoot + "/" + repoFile)
			if err != nil {
				glog.V(0).Info(err)
				continue
			}

			fileContent := string(body)
			hasOwners := strings.Contains(fileContent, "@owner")

			if !hasOwners {
				glog.V(0).Infoln(repoFile, owner)
				phpOwnerIndicator := fmt.Sprintf("<?php\n/**\n* TODO:write something\n* @owner %s\n**/\n", owner)
				ownerIndicator := fmt.Sprintf("/**\n* TODO:write something\n* @owner %s\n**/\n", owner)
				if strings.HasSuffix(repoFile, ".php") {
					phpIndex := strings.Index(fileContent, "<?php")
					if -1 != phpIndex {
						fileContent = strings.Replace(fileContent, "<?php", phpOwnerIndicator, 1)
					} else {
						phpIndex = strings.Index(fileContent, "<?")
						if -1 != phpIndex {
							fileContent = strings.Replace(fileContent, "<?", phpOwnerIndicator, 1)
						}
					}
				} else if strings.HasSuffix(repoFile, ".less") {
					fileContent = fmt.Sprintf("// @owner %s\n", owner) + fileContent
				} else {
					fileContent = ownerIndicator + fileContent
				}
				ioutil.WriteFile(gitRoot+"/"+repoFile, bytes.NewBufferString(fileContent).Bytes(), 1)
			}

		}
	}

}

func load(filename string, o interface{}) error {
	b, err := ioutil.ReadFile(filename)
	if err == nil {
		err = json.Unmarshal(b, &o)
	}
	return err
}

func main() {
	cfgFile := flag.String("cfg", "./github_webhook.cfg", "configuration file for github webhook")
	setOwners := flag.Bool("s", false, "reset owner to files")
	resetFile := flag.String("reset", "owners.csv", "reset owners file")
	gitRoot := flag.String("git", "/Users/jesse/houzz/c2", "root of git root")

	flag.Parse()
	defer glog.Flush()
	// Load gets your config from the json file,
	// and fills your struct with the option
	if err := load(*cfgFile, &cfg); err != nil {
		glog.V(0).Info("Failed to parse cfg data: %s", err)
	}
	host := flag.String("h", cfg.Host, "host of webhook service :45678")
	if *setOwners {
		setOwnersByFile(*resetFile, *gitRoot)

		return
	}

	glog.V(1).Info("github_webhook start")
	go readOwnersFile(cfg.OwnersFile)
	glog.V(1).Info("listeing is:", *host)
	startService(*host)
	glog.V(1).Info("githu_webhook end")
}

