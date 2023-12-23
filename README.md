# 온라인 오목 게임

이 프로젝트는 온라인 멀티플레이어 오목 게임을 구현한 프로젝트입니다. 오목은 가로, 세로, 대각선으로 다섯 개의 돌을 먼저 놓는 플레이어가 승리하는 보드 게임으로, 두 명의 플레이어가 번갈아가며 돌을 놓습니다.

## 기능

-   **실시간 멀티플레이어 게임**: 두 명의 플레이어가 인터넷을 통해 실시간으로 게임을 플레이합니다.
-   **WebSocket 통신**: 서버와 클라이언트 간에 WebSocket을 사용하여 실시간 통신을 수행합니다.
-   **게임 방 매칭**: 서버는 두 플레이어를 자동으로 매칭하여 게임 방을 생성합니다.

## 프로젝트 구성

이 프로젝트는 두 부분으로 구성됩니다.

1. **서버 측 코드 (Go)**: 온라인 오목 게임의 서버를 구현한 코드입니다. WebSocket 연결 관리, 게임 방 매칭, 게임 로직 등을 담당합니다.

2. **클라이언트 측 코드 (HTML, JavaScript)**: 웹 브라우저에서 동작하는 클라이언트 측 코드로, 사용자 인터페이스를 구성하고 WebSocket을 통해 서버와 실시간으로 통신하여 게임을 진행합니다.

## 실행 방법

1. 브라우저 접속: 온라인 오목 게임을 플레이하려면 [stonify5.com](https://stonify5.com) 주소로 웹 브라우저에 접속합니다.

2. 플레이: 웹 브라우저를 통해 접속하면 두 명의 플레이어가 매칭되어 게임이 시작됩니다. 각 플레이어는 번갈아가며 돌을 놓아서 승리를 목표로 합니다.
