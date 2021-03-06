package service_user

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mangmang/models"
	"github.com/mangmang/pkg/app"
	"github.com/mangmang/pkg/e"
	"github.com/mangmang/pkg/gredis"
	"github.com/mangmang/pkg/setting"
	"github.com/mangmang/pkg/utils"
	"github.com/unknwon/com"
	"net/http"
	"path"
	"time"
)

// 测试
func Test(c *gin.Context) {
	appG := app.New(c)
	sCode := com.ToStr(c.Param("k"))
	k := map[string]interface{}{
		"qin":  "1",
		"qin1": sCode,
	}
	d, _ := k["nickname"].(string)
	fmt.Print(d)
	appG.Response(http.StatusOK, e.SUCCESS, d)
	return
}

// 获取手机验证码
func GetVerificationCode(c *gin.Context) {
	appG := app.New(c)
	phone := c.Query("phone")
	if !utils.CheckPhone(phone) {
		appG.Response(http.StatusOK, e.MobileNumberError, nil)
		return
	}

	expireTime, err := gredis.Hget(phone, "expire_time")
	if err == nil {
		nowTime := time.Now()
		expireTime, _ := time.Parse("2006-01-02 15:04:05", string(expireTime))
		if nowTime.Unix()-expireTime.Unix() < 60 {
			appG.Response(http.StatusOK, e.FrequentOperation, nil)
			return
		}
	}
	//code := rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(1000000)
	code := "123456"
	err = gredis.Hset(phone, "code", code)
	if err != nil {
		appG.Response(http.StatusOK, e.FAIL, nil)
		return
	}
	err = gredis.Hset(phone, "expire_time", time.Now().Format("2006-01-02 15:04:05"))
	if err != nil {
		appG.Response(http.StatusOK, e.FAIL, nil)
		return
	}
	err = gredis.Expire(phone, 60*3)
	if err != nil {
		appG.Response(http.StatusOK, e.FAIL, nil)
		return
	}
	appG.Response(http.StatusOK, e.SUCCESS, nil)
	return
}

// 用户手机号注册
func PhoneRegister(c *gin.Context) {
	var obj struct {
		//Name      string `json:"name"binding:"max=20"`
		Phone     string `json:"phone"binding:"required,len=11"`
		PassWord1 string `json:"pass_word_1"binding:"required,min=6"`
		PassWord2 string `json:"pass_word_2"binding:"required,min=6"`
		Code      string `json:"code"binding:"required,len=6"`
	}
	appG := app.New(c)

	//参数解析失败
	if c.ShouldBindJSON(&obj) != nil {
		appG.Response(http.StatusOK, e.InvalidParameter, nil)
		return
	}

	// 两次密码不一致
	if obj.PassWord1 != obj.PassWord2 {
		appG.Response(http.StatusOK, e.InconsistentPassword, nil)
		return
	}

	// 验证码错误
	if !utils.CheckPhoneCode(obj.Phone, obj.Code, false) {
		appG.Response(http.StatusOK, e.VerificationCodeError, nil)
		return
	}
	// 验证手机号是否被注册
	if !models.IsExistPhone(obj.Phone) {
		appG.Response(http.StatusOK, e.PhoneNumberIsRegistered, nil)
		return
	}

	// 生成UUID错误
	userId := utils.GetUUID()

	newUser := &models.User{
		UserId: userId,
		Name:   utils.GetRandName(8, "A0"),
		Phone:  obj.Phone,
	}

	newLoginMethod := &models.UserLoginMethod{
		Id:             utils.GetUUID(),
		UserId:         userId,
		LoginType:      e.LoginPhone,
		Identification: obj.Phone,
		AccessCode:     utils.Md5Encrypt(obj.PassWord1),
	}

	if !models.Create(newUser, newLoginMethod) {
		appG.Response(http.StatusOK, e.NewFailed, nil)
		return
	}

	appG.Response(http.StatusOK, e.SUCCESS, newUser)
	return
}

// 用户密码登陆
func UserLoginAPW(c *gin.Context) {
	var obj struct {
		Phone    string `json:"phone"binding:"required,len=11"`
		PassWord string `json:"pass_word"binding:"required,min=6"`
	}
	appG := app.New(c)
	//参数解析失败
	if c.ShouldBindJSON(&obj) != nil {
		appG.Response(http.StatusOK, e.InvalidParameter, nil)
		return
	}

	// 判断用户登陆方式是否存在
	loginMethod, err := models.FindPhoneLoginMethod(obj.Phone)
	if err != nil {
		appG.Response(http.StatusOK, e.AccountOrPassWordErr, nil)
		return
	}

	// 判断账户密码是否正确
	if loginMethod.AccessCode != utils.Md5Encrypt(obj.PassWord) {
		appG.Response(http.StatusOK, e.AccountOrPassWordErr, nil)
		return
	}

	// 获取用户信息
	userInfo, err := models.FindUserIdInfo(loginMethod.UserId)
	if err != nil {
		appG.Response(http.StatusOK, e.AccountOrPassWordErr, nil)
		return
	}

	appG.Response(http.StatusOK, e.SUCCESS, userInfo)
	return

}

// 用户验证码登陆
func UserLoginAC(c *gin.Context) {
	var obj struct {
		Phone string `json:"phone"binding:"required,len=11"`
		Code  string `json:"code"binding:"required,len=6"`
	}
	appG := app.New(c)

	//参数解析失败
	if c.ShouldBindJSON(&obj) != nil {
		appG.Response(http.StatusOK, e.InvalidParameter, nil)
		return
	}

	// 验证码错误
	if !utils.CheckPhoneCode(obj.Phone, obj.Code, false) {
		appG.Response(http.StatusOK, e.VerificationCodeError, nil)
		return
	}

	// 判断用户登陆方式是否存在
	loginMethod, err := models.FindPhoneLoginMethod(obj.Phone)
	if err != nil {
		appG.Response(http.StatusOK, e.AccountOrPassWordErr, nil)
		return
	}

	// 获取用户信息
	userInfo, err := models.FindUserIdInfo(loginMethod.UserId)
	if err != nil {
		appG.Response(http.StatusOK, e.AccountOrPassWordErr, nil)
		return
	}

	appG.Response(http.StatusOK, e.SUCCESS, userInfo)
	return

}

// 用户修改密码
func ChangePassWord(c *gin.Context) {
	var obj struct {
		UserId      string `json:"user_id"binding:"required,uuid4"`
		OldPassWord string `json:"old_pass_word"binding:"required,min=6"`
		PassWord1   string `json:"pass_word_1"binding:"required,min=6"`
		PassWord2   string `json:"pass_word_2"binding:"required,min=6"`
	}
	appG := app.New(c)

	//参数解析失败
	if c.ShouldBindJSON(&obj) != nil {
		appG.Response(http.StatusOK, e.InvalidParameter, nil)
		return
	}
	// 两次密码不一致
	if obj.PassWord1 != obj.PassWord2 {
		appG.Response(http.StatusOK, e.InconsistentPassword, nil)
		return
	}

	// 查询密码登录方法是否存在
	loginMethod, err := models.FindUserIdLoginMethod(obj.UserId)
	if err != nil {
		appG.Response(http.StatusOK, e.FAIL, nil)
		return
	}
	// 比较老密码是否一致
	if loginMethod.AccessCode != utils.Md5Encrypt(obj.OldPassWord) {
		appG.Response(http.StatusOK, e.OldPasswordError, nil)
		return
	}
	// 更新密码失败
	if !models.UpdateUserPassWord(loginMethod, obj.PassWord1) {
		appG.Response(http.StatusOK, e.UpdateFailed, nil)
		return
	}

	appG.Response(http.StatusOK, e.SUCCESS, nil)
	return

}

// 用户修改个人信息
func ChangeUserInfo(c *gin.Context) {
	var obj struct {
		UserId       string         `json:"user_id"binding:"required,uuid4"`
		Name         string         `json:"name"binding:"required,max=20"`
		Email        string         `json:"email"binding:"omitempty,email"`
		Sex          int8           `json:"sex"`
		Introduction string         `json:"introduction"binding:"omitempty,max=255"`
		Position     string         `json:"position"binding:"omitempty,max=50"`
		Birthday     utils.JSONDate `json:"birthday"`
		Address      string         `json:"address"binding:"required,max=100"`
	}
	appG := app.New(c)
	//参数解析失败
	if c.ShouldBindJSON(&obj) != nil {
		appG.Response(http.StatusOK, e.InvalidParameter, nil)
		return
	}
	// 获取用户数据
	userInfo, err := models.FindUserIdInfo(obj.UserId)
	if err != nil {
		appG.Response(http.StatusOK, e.AccountDoesNotExist, nil)
		return
	}

	// 更新用户数据
	if !models.UpdateUserInfo(userInfo, obj) {
		appG.Response(http.StatusOK, e.UpdateFailed, nil)
		return
	}
	appG.Response(http.StatusOK, e.SUCCESS, nil)
	return

}

// 用户上传头像
func UploadAvatar(c *gin.Context) {
	appG := app.New(c)
	userId := c.PostForm("user_id")
	file, err := c.FormFile("file")

	// 用户ID错误或读取上传文件错误
	if userId == "" || err != nil {
		appG.Response(http.StatusOK, e.InvalidParameter, nil)
		return
	}

	// 判断文件格式是否正确
	fileSuffix := path.Ext(file.Filename)
	if fileSuffix != ".jpg" && fileSuffix != ".png" {
		appG.Response(http.StatusOK, e.ImageFormatIsIncorrect, nil)
		return
	}

	// 判断图片大小
	if file.Size/1024 > 40 {
		appG.Response(http.StatusOK, e.ImageByteIsTooLarge, nil)
		return
	}

	// 用户不存在
	userInfo, err := models.FindUserIdInfo(userId)
	if err != nil {
		appG.Response(http.StatusOK, e.AccountDoesNotExist, nil)
		return
	}

	// 拼接路径
	filePath := path.Join(setting.AppSetting.AvatarPath, userId+fileSuffix)
	err = c.SaveUploadedFile(file, filePath)
	if err != nil {
		appG.Response(http.StatusOK, e.FAIL, nil)
		return
	}

	// 更新头像路径
	if !models.UpdateUserInfo(userInfo, map[string]interface{}{"avatar_url": filePath}) {
		appG.Response(http.StatusOK, e.UpdateFailed, nil)
		return
	}

	appG.Response(http.StatusOK, e.SUCCESS, nil)
	return
}

// 搜索用户
func SearchUser(c *gin.Context) {
	appG := app.New(c)
	name := c.Query("name")

	if name == "" {
		appG.Response(http.StatusOK, e.InvalidParameter, nil)
		return
	}

	// 获取分页
	page, size := app.GetPageSize(c)
	users, total, err := models.FindNameUser(name, page, size)
	// 未找到数据
	if err != nil || len(users) == 0 {
		appG.Response(http.StatusOK, e.NoResourcesFound, nil)
		return
	}
	data := map[string]interface{}{
		"page":  page,
		"size":  size,
		"total": total,
		"users": users,
	}
	appG.Response(http.StatusOK, e.SUCCESS, data)
	return
}

// 获取用户信息
func GetUserInfo(c *gin.Context) {
	appG := app.New(c)
	key := c.Query("key")

	if key == "" {
		appG.Response(http.StatusOK, e.InvalidParameter, nil)
		return
	}

	// 获取用户信息
	userInfo, err := models.FindUserIdInfo(key)
	if err != nil {
		appG.Response(http.StatusOK, e.AccountOrPassWordErr, nil)
		return
	}

	appG.Response(http.StatusOK, e.SUCCESS, userInfo)
	return
}
