package com.lingqiao.server.common.web;

import com.lingqiao.server.common.errors.ErrorCode;
import com.lingqiao.server.common.errors.ProblemDetails;
import com.lingqiao.server.common.errors.ProblemDetailsFactory;
import com.lingqiao.server.common.errors.SidecarException;
import jakarta.servlet.http.HttpServletRequest;
import org.springframework.http.MediaType;
import org.springframework.http.ResponseEntity;
import org.springframework.http.converter.HttpMessageNotReadableException;
import org.springframework.web.bind.annotation.ExceptionHandler;
import org.springframework.web.bind.annotation.RestControllerAdvice;
import org.springframework.web.multipart.MaxUploadSizeExceededException;

@RestControllerAdvice
public class GlobalExceptionHandler {
    @ExceptionHandler(HttpMessageNotReadableException.class)
    public ResponseEntity<ProblemDetails> handleInvalidJson(
        HttpMessageNotReadableException ex, HttpServletRequest request
    ) {
        ProblemDetails problem = ProblemDetailsFactory.from(
            ErrorCode.INVALID_JSON, request.getRequestURI(), "invalid request body");
        return ResponseEntity.status(ErrorCode.INVALID_JSON.status())
            .contentType(MediaType.APPLICATION_PROBLEM_JSON)
            .body(problem);
    }

    @ExceptionHandler(MaxUploadSizeExceededException.class)
    public ResponseEntity<ProblemDetails> handleOversizedUpload(
        MaxUploadSizeExceededException ex, HttpServletRequest request
    ) {
        ProblemDetails problem = ProblemDetailsFactory.from(
            ErrorCode.BAD_REQUEST, request.getRequestURI(), "invalid package upload");
        return ResponseEntity.status(ErrorCode.BAD_REQUEST.status())
            .contentType(MediaType.APPLICATION_PROBLEM_JSON)
            .body(problem);
    }

    @ExceptionHandler(SidecarException.class)
    public ResponseEntity<ProblemDetails> handleSidecar(SidecarException ex, HttpServletRequest request) {
        ProblemDetails problem = ProblemDetailsFactory.from(ex.code(), request.getRequestURI(), ex.getMessage());
        return ResponseEntity.status(ex.code().status())
            .contentType(MediaType.APPLICATION_PROBLEM_JSON)
            .body(problem);
    }

    @ExceptionHandler(Exception.class)
    public ResponseEntity<ProblemDetails> handleUnexpected(Exception ex, HttpServletRequest request) {
        ProblemDetails problem = ProblemDetailsFactory.from(
            ErrorCode.INTERNAL_ERROR, request.getRequestURI(), "internal server error");
        return ResponseEntity.status(ErrorCode.INTERNAL_ERROR.status())
            .contentType(MediaType.APPLICATION_PROBLEM_JSON)
            .body(problem);
    }
}
