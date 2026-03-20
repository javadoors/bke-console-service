/*
 * Copyright (c) 2025Huawei Technologies Co., Ltd.
 * openFuyao is licensed under Mulan PSL v2.
 * You can use this software according to the terms and conditions of the Mulan PSL v2.
 * You may obtain a copy of Mulan PSL v2 at:
 *          http://license.coscl.org.cn/MulanPSL2
 * THIS SOFTWARE IS PROVIDED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OF ANY KIND,
 * EITHER EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO NON-INFRINGEMENT,
 * MERCHANTABILITY OR FIT FOR A PARTICULAR PURPOSE.
 * See the Mulan PSL v2 for more details.
 */

package util

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"console-service/pkg/constant"
)

func TestDecrypt(t *testing.T) {
	type args struct {
		cipherText []byte
		key        []byte
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "正常解密",
			args: func() args {
				key := []byte("12345678901234567890123456789012")
				plain := []byte("hello world")
				cipherText, _ := Encrypt(plain, key)
				return args{cipherText: cipherText, key: key}
			}(),
			want:    []byte("hello world"),
			wantErr: false,
		},
		{
			name:    "密钥长度错误",
			args:    args{cipherText: []byte("xxxx"), key: []byte("short")},
			want:    nil,
			wantErr: true,
		},
		{
			name: "密文被破坏",
			args: func() args {
				key := []byte("12345678901234567890123456789012")
				plain := []byte("hello world")
				cipherText, _ := Encrypt(plain, key)
				cipherText[10] ^= 0xFF // 篡改密文
				return args{cipherText: cipherText, key: key}
			}(),
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Decrypt(tt.args.cipherText, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Decrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Decrypt() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEncrypt(t *testing.T) {
	type args struct {
		plainText []byte
		key       []byte
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "正常加密",
			args:    args{plainText: []byte("hello world"), key: []byte("12345678901234567890123456789012")},
			wantErr: false,
		},
		{
			name:    "密钥长度错误",
			args:    args{plainText: []byte("hello world"), key: []byte("short")},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Encrypt(tt.args.plainText, tt.args.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encrypt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func getSymmetricKeyTestClient() kubernetes.Interface {
	return fake.NewSimpleClientset(
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "empty-secret",
				Namespace: constant.ConsoleServiceDefaultNamespace,
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "symmetric-key",
				Namespace: constant.ConsoleServiceDefaultNamespace,
			},
			Data: map[string][]byte{
				"console-service-symmetric-key": []byte("test-key"),
			},
		},
	)
}

func TestGetSecretSymmetricEncryptKey(t *testing.T) {
	client := getSymmetricKeyTestClient()

	tests := []struct {
		name       string
		secretName string
		want       []byte
		wantErr    bool
	}{
		{
			"TestNonExisting",
			"non-existing",
			nil,
			true,
		},
		{
			"TestNotContainingKey",
			"empty-secret",
			nil,
			true,
		},
		{
			"TestValidKey",
			"symmetric-key",
			[]byte("test-key"),
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetSecretSymmetricEncryptKey(client, tt.secretName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSecretSymmetricEncryptKey() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetSecretSymmetricEncryptKey() got = %v, want %v", got, tt.want)
			}
		})
	}
}
