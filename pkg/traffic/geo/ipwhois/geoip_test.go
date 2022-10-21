package ipwhois

import "testing"

func TestGetContinentCodeForIp(t *testing.T) {
	type args struct {
		ip string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "127.0.0.1",
			args: args{
				ip: "127.0.0.1",
			},
			want: "NA",
		},
		{
			name: "172.18.0.2",
			args: args{
				ip: "172.18.0.2",
			},
			want: "NA",
		},
		{
			name: "50.16.23.1",
			args: args{
				ip: "50.16.23.1",
			},
			want: "NA",
		},
		{
			name: "50.16.23.2",
			args: args{
				ip: "50.16.23.2",
			},
			want: "NA",
		},
		{
			name: "52.30.101.221",
			args: args{
				ip: "52.30.101.221",
			},
			want: "EU",
		},
		{
			name: "52.215.108.61",
			args: args{
				ip: "52.215.108.61",
			},
			want: "EU",
		},
		{
			name: "54.154.209.249",
			args: args{
				ip: "54.154.209.249",
			},
			want: "EU",
		},
		{
			name: "54.217.109.238",
			args: args{
				ip: "54.217.109.238",
			},
			want: "EU",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetContinentCodeForIp(tt.args.ip); got != tt.want {
				t.Errorf("GetContinentCodeForIp() = %v, want %v", got, tt.want)
			}
		})
	}
}
