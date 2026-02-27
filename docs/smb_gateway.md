# SMB 게이트웨이 운영 가이드 (1차)

## 목적
Cohesion은 SMB 서버를 코어 프로세스에 내장하지 않습니다.

SMB 네트워크 드라이브 연결은 **외부 게이트웨이(Samba)** 로 구성합니다.

- Cohesion: 파일/권한의 원본 시스템
- Samba: SMB/CIFS 프로토콜 제공 계층

## 적용 범위
- 대상 이슈: `#161`
- 지원 목표:
  - Windows: `\\<host>\<share>` 연결
  - macOS: `smb://<host>/<share>` 연결

## 사전 조건
- Cohesion 서버와 Samba 게이트웨이가 같은 파일 경로에 접근 가능해야 함
- Samba 포트 `445/tcp`를 내부망에서만 허용
- 익명(guest) 접근 금지

## 권장 접근 제어 모델
Cohesion RBAC/Space 권한은 SMB와 자동 동기화되지 않습니다.

따라서 1차에서는 아래 정책을 권장합니다.

- share 기본값을 `read only = yes`로 시작
- 쓰기가 필요한 share만 `write list = <smb-user>`로 명시
- Space 단위로 share를 분리해 권한 경계를 맞춤

## Samba 예시 (Linux)
예시 파일: `/etc/samba/smb.conf`

```ini
[global]
  workgroup = WORKGROUP
  server string = Cohesion SMB Gateway
  security = user
  map to guest = never
  smb encrypt = desired

[cohesion_space_docs]
  path = /srv/cohesion/spaces/docs
  browseable = yes
  read only = yes
  valid users = @cohesion_docs

[cohesion_space_team]
  path = /srv/cohesion/spaces/team
  browseable = yes
  read only = yes
  valid users = @cohesion_team
  write list = @cohesion_team_rw
```

사용자/그룹 예시:

```bash
# 그룹 생성
sudo groupadd cohesion_docs
sudo groupadd cohesion_team
sudo groupadd cohesion_team_rw

# SMB 사용자 추가
sudo useradd -M -s /usr/sbin/nologin cohesion_user
sudo smbpasswd -a cohesion_user

# 그룹 할당
sudo usermod -aG cohesion_team,cohesion_team_rw cohesion_user
```

서비스 반영:

```bash
sudo testparm
sudo systemctl restart smbd
sudo systemctl status smbd --no-pager
```

## 클라이언트 연결 절차
### Windows
1. 파일 탐색기 > 네트워크 드라이브 연결
2. 폴더: `\\<host>\cohesion_space_team`
3. 다른 자격 증명 사용 체크 후 SMB 계정 입력
4. 읽기/쓰기 권한 시나리오 검증

### macOS
1. Finder > 이동 > 서버에 연결
2. 주소: `smb://<host>/cohesion_space_team`
3. 등록된 사용자로 로그인
4. 읽기/쓰기 권한 시나리오 검증

## 검증 체크리스트
- [ ] 읽기 전용 share에서 생성/수정/삭제가 차단되는지
- [ ] 쓰기 권한 share에서 생성/수정/삭제가 가능한지
- [ ] 잘못된 계정으로 로그인 시 거부되는지
- [ ] Cohesion UI와 SMB에서 동일 경로 변경이 상호 반영되는지

## 알려진 제약
- SMB 접근 제어는 Samba 계층에서 별도 운영해야 합니다.
- Cohesion의 사용자/Role/Space 권한이 SMB 계정으로 자동 매핑되지는 않습니다.
- 운영 시 최소 권한 원칙(기본 read-only, 필요한 share만 write 허용)을 유지하세요.
