apps/backend/
├── internal/
│   │
│   ├── webdav/                   # WebDAV 서버
│   │   └── server.go             #    - WebDAV 요청을 받아 핵심 서비스 호출
│   │
│   ├── ftp/                      # FTP 서버
│   │   └── server.go             #    - FTP 명령어를 받아 핵심 서비스 호출
│   │
│   ├── file/                     # "파일" 핵심 비즈니스 로직 (프로토콜에 독립적)
│   │   ├── api.go                #    - 파일 기능의 REST API 핸들러 및 라우팅
│   │   ├── file.go               #    - File 모델(struct) 정의
│   │   └── service.go            #    - 파일 생성, 읽기, 삭제, 이동 등
│   │
│   ├── share/                    # "공유" 핵심 비즈니스 로직
│   │   ├── api.go                #    - 공유 기능의 REST API 핸들러 및 라우팅
│   │   ├── share.go              #    - Share 모델(struct) 정의
│   │   └── service.go            #    - 공유 폴더 생성, 권한 관리 등
│   │
│   ├── user/                     # "사용자" 핵심 비즈니스 로직
│   │   ├── api.go                #    - 사용자 기능의 REST API 핸들러 및 라우팅
│   │   ├── user.go
│   │   └── service.go
│   │
│   ├── storage/                  # 실제 파일 저장소 (추상화 계층)
│   │   ├── interface.go          #    - FileStorage 인터페이스 정의 (Open, Save, Delete 등)
│   │   ├── filesystem.go         #    - 로컬 파일 시스템 구현체
│   │   └── s3.go                 #    - (선택) S3 같은 클라우드 스토리지 구현체
│   │
│   ├── platform/                 # 공통 기반 기술
│   │   ├── database/             #    - 데이터베이스 연결 및 관리
│   │   │   └── db.go             #    - SQLite 연결 및 DB 커넥션 풀 생성
│   │   └── auth/                 #    - 인증/인가 로직 (모든 프로토콜에서 사용)
│   │
│   └── spa/                      # SPA 프론트엔드 서빙 핸들러
│       └── handler.go
│
├── main.go                     # 애플리케이션 시작점. 모든 서버(API, WebDAV, FTP)를 실행.
├── embed_dev.go
├── embed_prod.go
└── go.mod