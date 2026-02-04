**항상 docs/master_rule.md를 먼저 읽기**

---

## Serena MCP 필수 사용 규칙

이 프로젝트에서는 **Serena MCP를 필수로 사용**해야 합니다.

### [금지] 사항
- `Read` 툴로 파일 전체 읽기 (코드 파일의 경우)
- `Edit` 툴로 코드 수정하기
- `Write` 툴로 새 코드 파일 생성하기
- `Grep` 툴로 코드 검색하기

### [필수] 사용 패턴

#### 1. 코드 탐색
```
[X] Read("MainLayout/index.tsx")
[O] get_symbols_overview("MainLayout/index.tsx", depth=1)
[O] find_symbol("PageLayout", include_body=true)
```

#### 2. 코드 수정
```
[X] Edit(file_path, old_string, new_string)
[O] replace_symbol_body(name_path="PageLayout", body="...")
[O] insert_after_symbol(name_path="imports", body="...")
```

#### 3. 코드 검색
```
[X] Grep(pattern="Space")
[O] find_symbol("Space", substring_matching=true)
[O] search_for_pattern(substring_pattern="Space.*interface")
```

#### 4. 의존성 확인
```
[O] find_referencing_symbols(name_path="Space", relative_path="types.ts")
```

### 작업 흐름 (필수)

**Step 1: 구조 파악**
```typescript
get_symbols_overview(relative_path, depth=1)
```

**Step 2: 필요한 심볼만 읽기**
```typescript
find_symbol(name_path_pattern, include_body=true)
```

**Step 3: 의존성 확인 (필요시)**
```typescript
find_referencing_symbols(name_path, relative_path)
```

**Step 4: 심볼 단위 수정**
```typescript
replace_symbol_body(name_path, relative_path, body)
```

### 핵심 원칙
1. **절대 파일 전체를 읽지 않는다** - 필요한 심볼만 읽는다
2. **문자열 편집을 하지 않는다** - 심볼 단위로 수정한다
3. **토큰을 절약한다** - overview → symbol → edit 순서로 최소한만 읽는다

### 예외 사항
- 설정 파일(JSON, YAML, MD 등): 일반 Read/Edit 허용
- 매우 짧은 파일(< 50줄): 일반 Read 허용
- 비코드 파일: 일반 툴 사용 가능
