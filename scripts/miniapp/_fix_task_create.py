# -*- coding: utf-8 -*-
import os

base = r'C:\go\go_code\src\go_projects\praProject1\scripts\miniapp\pages\task'

# WXML
wxml = '''<wxs src="../../utils/format.wxs" module="fmt"/>
<view class="create-page">
  <scroll-view scroll-y class="create-scroll">
    <!-- \u4efb\u52a1\u7c7b\u578b -->
    <view class="create-section">
      <view class="section-title">\u4efb\u52a1\u7c7b\u578b</view>
      <view class="type-selector">
        <view wx:for="{{types}}" wx:key="*this" class="type-option {{taskType===item?'active':''}}" data-type="{{item}}" bindtap="onTypeChange">
          <text class="type-option-text">{{typeText[item]}}</text>
        </view>
      </view>
    </view>
    <!-- \u4efb\u52a1\u4fe1\u606f -->
    <view class="create-section">
      <view class="section-title">\u4efb\u52a1\u4fe1\u606f</view>
      <view class="form-group">
        <text class="field-label required">\u6807\u9898</text>
        <input class="form-input" placeholder="\u8bf7\u586b\u5199\u4efb\u52a1\u6807\u9898" value="{{title}}" data-field="title" bindinput="onInput" />
      </view>
      <view class="form-group">
        <text class="field-label">\u4efb\u52a1\u63cf\u8ff0</text>
        <textarea class="form-textarea" placeholder="\u8be6\u7ec6\u63cf\u8ff0\u4f60\u7684\u4efb\u52a1\u9700\u6c42..." value="{{description}}" data-field="description" bindinput="onInput"></textarea>
      </view>
    </view>
    <!-- \u8be6\u7ec6\u8bf4\u660e -->
    <view class="create-section">
      <view class="section-title">\u8be6\u7ec6\u8bf4\u660e</view>
      <view class="form-group">
        <text class="field-label">\u5730\u70b9</text>
        <input class="form-input" placeholder="\u586b\u5199\u4efb\u52a1\u5730\u70b9\uff08\u9009\u586b\uff09" value="{{location}}" data-field="location" bindinput="onInput" />
      </view>
      <view class="form-group">
        <text class="field-label">\u62a5\u916c\u63cf\u8ff0</text>
        <input class="form-input" placeholder="\u586b\u5199\u62a5\u916c\u8be6\u60c5\uff08\u9009\u586b\uff09" value="{{rewardDesc}}" data-field="rewardDesc" bindinput="onInput" />
      </view>
      <view class="form-group">
        <text class="field-label required">\u8054\u7cfb\u65b9\u5f0f</text>
        <input class="form-input" placeholder="\u8bf7\u586b\u5199\u624b\u673a\u53f7/\u5fae\u4fe1\u7b49\u8054\u7cfb\u65b9\u5f0f" value="{{contact}}" data-field="contact" bindinput="onInput" />
      </view>
      <view class="form-group">
        <text class="field-label">\u5907\u6ce8</text>
        <input class="form-input" placeholder="\u5176\u4ed6\u9700\u8981\u8bf4\u660e\u7684\u5185\u5bb9\uff08\u9009\u586b\uff09" value="{{note}}" data-field="note" bindinput="onInput" />
      </view>
    </view>
    <view class="create-submit">
      <button class="btn-primary submit-btn" bindtap="onSubmit" disabled="{{submitting}}" loading="{{submitting}}">\u53d1\u5e03\u4efb\u52a1</button>
    </view>
  </scroll-view>
</view>'''

with open(os.path.join(base, 'create.wxml'), 'w', encoding='utf-8') as f:
    f.write(wxml)
print('WXML OK')

# WXSS
wxss = '''.create-page{min-height:100vh;background:var(--white)}
.create-scroll{padding-bottom:60rpx}
.create-section{padding:32rpx 36rpx;border-bottom:12rpx solid var(--bg)}
.section-title{font-size:30rpx;font-weight:600;color:var(--text-primary);margin-bottom:24rpx;padding-left:8rpx}
.type-selector{display:flex;gap:16rpx;padding:0 8rpx}
.type-option{flex:1;text-align:center;padding:20rpx 0;border-radius:14rpx;border:2rpx solid var(--border);font-size:26rpx;color:var(--text-secondary);transition:all .2s}
.type-option.active{border-color:var(--primary);color:var(--primary);background:var(--primary-light)}
.form-group{margin-bottom:28rpx}
.form-group:last-child{margin-bottom:0}
.field-label{display:block;font-size:28rpx;color:var(--text-secondary);margin-bottom:12rpx;padding-left:4rpx}
.field-label.required::after{content:" *";color:var(--danger)}
.form-textarea{width:100%;min-height:200rpx;font-size:28rpx;border:2rpx solid var(--border);background:var(--white);border-radius:12rpx;padding:20rpx 24rpx;box-sizing:border-box;line-height:1.6}
.form-textarea:focus{border-color:var(--primary)}
.form-input{width:100%;padding:22rpx 0;font-size:30rpx;border-bottom:2rpx solid var(--border);box-sizing:border-box}
.form-input:focus{border-bottom-color:var(--primary)}
.create-submit{padding:60rpx 36rpx 80rpx}
.submit-btn{height:96rpx;font-size:32rpx;border-radius:16rpx!important;font-weight:500}'''

with open(os.path.join(base, 'create.wxss'), 'w', encoding='utf-8') as f:
    f.write(wxss)
print('WXSS OK')

# JS
js = '''const api=require('../../utils/api')
const {TASK_TYPE_TEXT}=require('../../utils/constants')
Page({
  data:{types:[1,2,3],taskType:1,title:'',description:'',location:'',rewardDesc:'',contact:'',note:'',expiredAt:'',submitting:false,typeText:TASK_TYPE_TEXT},
  onTypeChange(e){this.setData({taskType:parseInt(e.currentTarget.dataset.type)})},
  onInput(e){const f=e.currentTarget.dataset.field;if(f)this.setData({[f]:e.detail.value})},
  onSubmit(){if(!this.data.title.trim())return wx.showToast({title:'\u8bf7\u8f93\u5165\u4efb\u52a1\u6807\u9898',icon:'none'})
  if(!this.data.contact.trim())return wx.showToast({title:'\u8bf7\u586b\u5199\u8054\u7cfb\u65b9\u5f0f',icon:'none'})
    this.setData({submitting:true})
    api.createTask({task_type:this.data.taskType,title:this.data.title.trim(),description:this.data.description.trim(),location:this.data.location.trim(),reward_desc:this.data.rewardDesc.trim(),contact:this.data.contact.trim(),note:this.data.note.trim(),expired_at:this.data.expiredAt?parseInt(this.data.expiredAt):0}).then(()=>{this.setData({submitting:false});wx.showToast({title:'\u53d1\u5e03\u6210\u529f',icon:'success'});wx.switchTab({url:'/pages/tasks/tasks'})}).catch(()=>{this.setData({submitting:false});wx.showToast({title:'\u53d1\u5e03\u5931\u8d25',icon:'none'})})}
})'''

with open(os.path.join(base, 'create.js'), 'w', encoding='utf-8') as f:
    f.write(js)
print('JS OK')

print('All files written successfully')
